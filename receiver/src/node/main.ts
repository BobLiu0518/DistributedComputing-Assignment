import { createInterface } from 'node:readline';
import { io, Socket } from 'socket.io-client';
import { consumeLoop, setAuth, verifyConnection } from '../shared/consumer.js';
import { DISPATCH_RULES, PROCESSOR_LABELS, PROCESSOR_ACTIONS, consumerQueueName, DASHBOARD_URL } from '../shared/constants.js';
import { createActionsForMessage, executeAction } from '../handlers/executor.js';
import type { EmergencyMessage, ProcessorType, NodeReport, EmergencyAction } from '../shared/types.js';

const VALID_PROCESSORS: readonly ProcessorType[] = [
  'gate-controller', 'sms-sender', 'alarm-controller',
  'power-controller', 'valve-controller', 'medical-dispatcher',
];

function parseArgs(): { processor: ProcessorType; username: string; password: string } {
  const args = process.argv.slice(2);
  const processorArg = args.find((a) => a.startsWith('--processor='))?.split('=')[1] as ProcessorType | undefined;
  const userArg = args.find((a) => a.startsWith('--user='))?.split('=')[1];
  const passArg =
    args.find((a) => a.startsWith('--password='))?.split('=')[1] ??
    args.find((a) => a.startsWith('--pass='))?.split('=')[1];

  if (!processorArg || !VALID_PROCESSORS.includes(processorArg)) {
    console.error('用法: pnpm start --processor=gate-controller|sms-sender|alarm-controller|power-controller|valve-controller|medical-dispatcher --user=xxx --password=xxx');
    process.exit(1);
  }

  if (!userArg || !passArg) {
    console.error('请提供 ActiveMQ 凭证: --user=xxx --password=xxx');
    process.exit(1);
  }

  return { processor: processorArg, username: userArg, password: passArg };
}

async function main(): Promise<void> {
  const { processor, username, password } = parseArgs();
  const nodeLabel = PROCESSOR_LABELS[processor];
  const myActions = PROCESSOR_ACTIONS[processor];
  const clientId = `processor-${processor}-${Date.now()}`;

  console.log(`\n=== MQ 订阅处理器 - ${nodeLabel} ===`);
  console.log(`处理器ID: ${clientId}`);
  console.log(`负责操作: ${myActions.join(', ')}`);

  setAuth(username, password);
  const verify = await verifyConnection();
  if (!verify.ok) {
    console.error(`ActiveMQ 认证失败: ${verify.message}`);
    process.exit(1);
  }
  console.log(`ActiveMQ: ${verify.message}`);

  const socket: Socket = io(DASHBOARD_URL, {
    reconnection: true,
    reconnectionDelay: 1000,
    reconnectionDelayMax: 5000,
    reconnectionAttempts: Infinity,
  });

  socket.on('connect', () => console.log(`[${nodeLabel}] 已连接 Dashboard`));
  socket.on('disconnect', (reason) => console.log(`[${nodeLabel}] Dashboard 断开 (${reason})`));
  socket.on('connect_error', (err) => console.log(`[${nodeLabel}] Dashboard 连接失败: ${err.message}`));

  function reportAction(action: EmergencyAction): void {
    const report: NodeReport = { node: processor, nodeLabel, action };
    socket.emit('node:action', report);
  }

  const processedMsgIds = new Set<string>();
  const MAX_PROCESSED_IDS = 50000;

  async function handleMessage(msg: EmergencyMessage): Promise<void> {
    if (processedMsgIds.has(msg.msgid)) {
      return;
    }
    if (processedMsgIds.size >= MAX_PROCESSED_IDS) {
      const toDelete = Math.floor(MAX_PROCESSED_IDS / 2);
      let count = 0;
      for (const id of processedMsgIds) {
        processedMsgIds.delete(id);
        if (++count >= toDelete) break;
      }
    }
    processedMsgIds.add(msg.msgid);

    const rule = DISPATCH_RULES[msg.type];
    if (!rule) {
      return;
    }

    // 从分发规则中筛选本处理器负责的操作
    const processorActions = rule.actions.filter((a) => myActions.includes(a.type));
    if (processorActions.length === 0) {
      return;
    }

    const actions = createActionsForMessage(msg, processor, processorActions);
    console.log(`[${nodeLabel}] 收到 #${msg.seq} ${msg.type} @ ${msg.location} (${msg.msgid.slice(0, 8)}) → ${actions.length} 个操作`);

    for (const action of actions) {
      reportAction(action);
    }

    for (const action of actions) {
      await executeAction(action, (updated) => {
        reportAction(updated);
        const statusIcon = updated.status === 'done' ? '✓' : updated.status === 'error' ? '✗' : '…';
        console.log(`  ${statusIcon} ${updated.label} [${updated.status}]`);
      });
    }
  }

  const abortController = new AbortController();

  process.on('SIGINT', () => {
    console.log(`\n正在关闭 ${nodeLabel}...`);
    abortController.abort();
    socket.disconnect();
    process.exit(0);
  });

  const rl = createInterface({ input: process.stdin, output: process.stdout });
  rl.on('line', (line) => {
    if (line.trim() === 'quit' || line.trim() === 'exit') {
      abortController.abort();
      socket.disconnect();
      rl.close();
      process.exit(0);
    }
  });

  console.log('输入 quit/exit 退出，Ctrl+C 亦可');
  console.log('等待接收紧急消息...\n');

  const queueName = consumerQueueName(processor);
  console.log(`消费队列: ${queueName}`);

  await consumeLoop(queueName, clientId, handleMessage, abortController.signal);
}

main().catch((err) => {
  console.error('Fatal error:', err);
  process.exit(1);
});
