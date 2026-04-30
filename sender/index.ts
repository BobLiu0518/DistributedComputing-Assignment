import * as blessed from 'blessed';
import { randomUUID } from 'node:crypto';
import locationCategories from '../data/locations.json';
import emergencies from '../data/emergencies.json';

const ACTIVEMQ_BASE_URL = 'https://mq.usst2.bobliu.tech';
const TOPIC_NAME = 'VirtualTopic.campus.emergency';
const BATCH_SIZE = 50;
const INTERVAL_MS = 1000;
const PROMPT = 'MQ-Sensor > ';

const LOCATIONS = Object.values(locationCategories).flat();

const TYPE_LOCATION_MAPPING = Object.fromEntries(
  Object.entries(emergencies).map(([type, categories]) => {
    if (categories.includes('所有')) {
      return [type, LOCATIONS];
    }
    const locations = categories.flatMap((cat) => locationCategories[cat as keyof typeof locationCategories] ?? []);
    return [type, locations];
  }),
);

const MESSAGE_TYPES = Object.keys(TYPE_LOCATION_MAPPING);

type MessageType = keyof typeof TYPE_LOCATION_MAPPING;
type Location = (typeof LOCATIONS)[number];

interface Message {
  type: MessageType;
  location: Location;
  timestamp: number;
  seq: number;
  msgid: string;
}

interface SendResult {
  seq: number;
  msgid: string;
  type: MessageType;
  location: Location;
  ok: boolean;
  status?: number;
  errorMessage?: string;
}

let running = false;
let messageId = 0;
let timer: ReturnType<typeof setInterval> | null = null;
let authHeader = '';

let screen: blessed.Widgets.Screen | null = null;
let logWidget: blessed.Widgets.Log | null = null;

function randomFrom<T>(arr: readonly T[]): T {
  return arr[Math.floor(Math.random() * arr.length)];
}

function newMessage(): Message {
  const type = randomFrom(MESSAGE_TYPES);
  const location = randomFrom(TYPE_LOCATION_MAPPING[type]);
  return {
    type,
    location,
    timestamp: Date.now(),
    seq: messageId++,
    msgid: randomUUID(),
  };
}

function ts(): string {
  const d = new Date();
  const h = d.getHours().toString().padStart(2, '0');
  const m = d.getMinutes().toString().padStart(2, '0');
  const s = d.getSeconds().toString().padStart(2, '0');
  const ms = d.getMilliseconds().toString().padStart(3, '0');
  return `${h}:${m}:${s}.${ms}`;
}

function logLine(text: string): void {
  if (logWidget) {
    logWidget.log(text);
  }
}

function refresh(): void {
  if (screen) {
    screen.render();
  }
}

async function sendOne(msg: Message): Promise<SendResult> {
  const url = `${ACTIVEMQ_BASE_URL}/api/message/${TOPIC_NAME}?type=topic`;

  try {
    const res = await fetch(url, {
      method: 'POST',
      headers: {
        Authorization: authHeader,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(msg),
    });
    return { seq: msg.seq, msgid: msg.msgid, type: msg.type, location: msg.location, ok: res.ok, status: res.status };
  } catch (err: unknown) {
    const errMsg = err instanceof Error ? err.message : String(err);
    return { seq: msg.seq, msgid: msg.msgid, type: msg.type, location: msg.location, ok: false, errorMessage: errMsg };
  }
}

function formatLine(r: SendResult): string {
  const t = ts();
  if (r.errorMessage) {
    return `{red-fg}[${t}] #${r.seq} ERROR: ${r.type} @ ${r.location} - ${r.errorMessage}{/red-fg}`;
  }
  if (r.ok) {
    return `{green-fg}[${t}] #${r.seq} OK: ${r.type} @ ${r.location}{/green-fg}`;
  }
  return `{red-fg}[${t}] #${r.seq} FAIL: ${r.type} @ ${r.location} (HTTP ${r.status}){/red-fg}`;
}

async function sendBatch(): Promise<void> {
  const tasks: Promise<void>[] = [];
  for (let i = 0; i < BATCH_SIZE; i++) {
    const msg = newMessage();
    tasks.push(
      sendOne(msg).then((r) => {
        logLine(formatLine(r));
        refresh();
      }),
    );
  }
  await Promise.all(tasks);
}

function cmdStart(): void {
  if (running) {
    logLine(`{yellow-fg}[${ts()}] Already running{/yellow-fg}`);
    refresh();
    return;
  }
  running = true;
  logLine(`{green-fg}[${ts()}] Started sending (${BATCH_SIZE} msg/s){/green-fg}`);
  sendBatch();
  timer = setInterval(() => {
    sendBatch();
  }, INTERVAL_MS);
  refresh();
}

function cmdStop(): void {
  if (!running) {
    logLine(`{yellow-fg}[${ts()}] Already stopped{/yellow-fg}`);
    refresh();
    return;
  }
  if (timer !== null) {
    clearInterval(timer);
    timer = null;
  }
  running = false;
  logLine(`{yellow-fg}[${ts()}] Stopped{/yellow-fg}`);
  refresh();
}

function cmdExit(): void {
  if (timer !== null) {
    clearInterval(timer);
    timer = null;
  }
  running = false;
  if (screen) {
    screen.destroy();
  }
  process.exit(0);
}

function buildMainUI(): void {
  if (!screen) return;

  logWidget = blessed.log({
    parent: screen,
    top: 0,
    left: 0,
    width: '100%',
    height: '100%-3',
    border: 'line',
    style: { border: { fg: 'cyan' }, scrollbar: { bg: 'white', fg: 'gray' } },
    tags: true,
    scrollable: true,
    mouse: true,
    keys: true,
    scrollbar: { ch: ' ' },
    scrollback: 100000,
    scrollOnInput: true,
  });

  logLine('{cyan-fg}MQ Sensor{/cyan-fg}');
  logLine(`{cyan-fg}Target: ${ACTIVEMQ_BASE_URL}{/cyan-fg}`);
  logLine(`{cyan-fg}Topic:  ${TOPIC_NAME}{/cyan-fg}`);
  logLine('{cyan-fg}---{/cyan-fg}');
  logLine('{white-fg}Commands:{/white-fg}');
  logLine('  {green-fg}start{/green-fg} - Begin sending messages');
  logLine('  {yellow-fg}stop{/yellow-fg}  - Stop sending messages');
  logLine('  {red-fg}exit{/red-fg}  - Quit{/red-fg}');
  logLine('{cyan-fg}---{/cyan-fg}');

  const cmdBox = blessed.box({
    parent: screen,
    bottom: 0,
    left: 0,
    width: '100%',
    height: 3,
    border: 'line',
    style: { border: { fg: 'green' }, fg: 'white', bg: 'black' },
    content: PROMPT + '\u2588',
  });

  let cmdBuffer = '';

  function renderCmd(): void {
    cmdBox.setContent(PROMPT + cmdBuffer + '\u2588');
    refresh();
  }

  cmdBox.focus();
  cmdBox.on('keypress', (ch: string, key: blessed.Widgets.Events.IKeyEventArg) => {
    if (key.name === 'up' || key.name === 'down' || key.name === 'pageup' || key.name === 'pagedown') {
      if (logWidget) {
        if (key.name === 'up') logWidget.scroll(-1);
        if (key.name === 'down') logWidget.scroll(1);
        if (key.name === 'pageup') logWidget.scroll(-10);
        if (key.name === 'pagedown') logWidget.scroll(10);
        refresh();
      }
      return;
    }
    if (key.name === 'backspace') {
      if (cmdBuffer.length > 0) {
        cmdBuffer = cmdBuffer.slice(0, -1);
        renderCmd();
      }
      return;
    }
    if (key.name === 'enter' || key.name === 'return') {
      const cmd = cmdBuffer.trim();
      cmdBuffer = '';
      renderCmd();
      switch (cmd) {
        case 'start':
          cmdStart();
          break;
        case 'stop':
          cmdStop();
          break;
        case 'exit':
          cmdExit();
          return;
      }
      return;
    }
    if (ch && ch.length === 1 && !key.ctrl && !key.meta) {
      cmdBuffer += ch;
      renderCmd();
    }
  });

  screen.key(['C-c'], () => {
    cmdExit();
  });

  refresh();
}

async function verifyCredentials(auth: string, onSuccess: () => void, onFailure: (msg: string) => void): Promise<void> {
  try {
    const res = await fetch(`${ACTIVEMQ_BASE_URL}/api/jolokia/`, {
      method: 'GET',
      headers: {
        Authorization: auth,
      },
    });
    if (res.status === 401 || res.status === 403) {
      onFailure(`Authentication failed (HTTP ${res.status})`);
      return;
    }
    if (res.ok) {
      onSuccess();
      return;
    }
    onFailure(`Unexpected response (HTTP ${res.status})`);
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err);
    onFailure(`Connection failed: ${msg}`);
  }
}

type LoginField = 'username' | 'password';

interface LoginInput {
  box: blessed.Widgets.BoxElement;
  getBuffer: () => string;
  setBuffer: (v: string) => void;
  render: (active: boolean) => void;
}

function makeLoginInput(parent: blessed.Widgets.Node, opts: { top: number; left: number; width: number; censor?: boolean }): LoginInput {
  let buffer = '';

  const box = blessed.box({
    parent,
    top: opts.top,
    left: opts.left,
    width: opts.width,
    height: 1,
    content: '\u2588',
    style: { fg: 'white', bg: 'black' },
  });

  function render(active: boolean): void {
    let display: string;
    if (opts.censor) {
      display = '*'.repeat(buffer.length);
    } else {
      display = buffer;
    }
    if (active) {
      display += '\u2588';
    }
    box.setContent(display);
  }

  box.on('keypress', (ch: string, key: blessed.Widgets.Events.IKeyEventArg) => {
    if (key.name === 'backspace') {
      if (buffer.length > 0) {
        buffer = buffer.slice(0, -1);
        render(true);
        refresh();
      }
      return;
    }
    if (key.name === 'enter' || key.name === 'return') {
      box.emit('submit');
      return;
    }
    if (ch && ch.length === 1 && !key.ctrl && !key.meta) {
      buffer += ch;
      render(true);
      refresh();
    }
  });

  return {
    box,
    getBuffer: () => buffer,
    setBuffer: (v: string) => {
      buffer = v;
    },
    render,
  };
}

function showLogin(): void {
  if (!screen) return;

  const formWidth = 46;
  const formHeight = 10;

  const form = blessed.box({
    parent: screen,
    left: 'center',
    top: 'center',
    width: formWidth,
    height: formHeight,
    border: 'line',
    style: { border: { fg: 'yellow' } },
    label: ' MQ Sensor Login ',
    tags: true,
  });

  blessed.text({
    parent: form,
    top: 2,
    left: 3,
    content: 'Username:',
    style: { fg: 'white' },
  });

  const userInput = makeLoginInput(form, { top: 2, left: 14, width: 27 });

  blessed.text({
    parent: form,
    top: 4,
    left: 3,
    content: 'Password:',
    style: { fg: 'white' },
  });

  const passInput = makeLoginInput(form, { top: 4, left: 14, width: 27, censor: true });

  const statusText = blessed.text({
    parent: form,
    top: 7,
    left: 3,
    width: formWidth - 6,
    content: '',
    style: { fg: 'red' },
    tags: true,
  });

  let activeField: LoginField = 'username';

  userInput.render(true);
  passInput.render(false);
  userInput.box.focus();

  function switchTo(field: LoginField): void {
    activeField = field;
    if (field === 'username') {
      userInput.render(true);
      passInput.render(false);
      userInput.box.focus();
    } else {
      passInput.render(true);
      userInput.render(false);
      passInput.box.focus();
    }
    refresh();
  }

  userInput.box.on('submit', () => {
    if (userInput.getBuffer().length > 0) {
      switchTo('password');
    }
  });

  passInput.box.on('submit', () => {
    const username = userInput.getBuffer();
    const password = passInput.getBuffer();

    if (password.length === 0) return;
    if (username.length === 0) {
      statusText.setContent('{red-fg}Please enter username first{/red-fg}');
      refresh();
      return;
    }

    const auth = `Basic ${Buffer.from(`${username}:${password}`).toString('base64')}`;
    statusText.setContent('{yellow-fg}Verifying credentials...{/yellow-fg}');
    refresh();

    verifyCredentials(
      auth,
      () => {
        authHeader = auth;
        form.destroy();
        buildMainUI();
      },
      (msg: string) => {
        statusText.setContent(`{red-fg}${msg}{/red-fg}`);
        userInput.setBuffer('');
        passInput.setBuffer('');
        switchTo('username');
      },
    );
  });

  userInput.box.key('tab', () => {
    if (userInput.getBuffer().length > 0) {
      switchTo('password');
    }
  });

  passInput.box.key('tab', () => {
    switchTo('username');
  });

  screen.key(['C-c'], () => {
    cmdExit();
  });

  refresh();
}

screen = blessed.screen({
  smartCSR: true,
  fullUnicode: true,
  title: 'MQ Sensor',
});

process.stdout.write('\x1b[?25l');
screen.on('render', () => {
  process.stdout.write('\x1b[?25l');
});

process.on('SIGINT', () => {
  cmdExit();
});

showLogin();
