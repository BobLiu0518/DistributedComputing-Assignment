import { createInterface } from 'node:readline';
import { io, Socket } from 'socket.io-client';
import { consumeLoop, setAuth, verifyConnection } from '../shared/consumer.js';
import { DISPATCH_RULES, NODE_LABELS, DASHBOARD_URL } from '../shared/constants.js';
import { createActionsForMessage, executeAction } from '../handlers/executor.js';
import type { EmergencyMessage, NodeRole, NodeReport, EmergencyAction } from '../shared/types.js';

function parseArgs(): { node: NodeRole; username: string; password: string } {
  const args = process.argv.slice(2);
  const nodeArg = args.find((a) => a.startsWith('--node='))?.split('=')[1] as NodeRole | undefined;
  const userArg = args.find((a) => a.startsWith('--user='))?.split('=')[1];
  const passArg = args.find((a) => a.startsWith('--pass='))?.split('=')[1];

  if (!nodeArg || !['security', 'medical'].includes(nodeArg)) {
    console.error('用法: pnpm start --node=security|medical --user=xxx --pass=xxx');
    process.exit(1);
  }

  if (!userArg || !passArg) {
    console.error('请提供 ActiveMQ 凭证: --user=xxx --pass=xxx');
    process.exit(1);
  }

  return { node: nodeArg, username: userArg, password: passArg };
}

async function main(): Promise<void> {
  const { node, username, password } = parseArgs();
  const nodeLabel = NODE_LABELS[node];
  const clientId = `receiver-${node}-${Date.now()}`;

  console.log(`\n=== MQ 接收端 - ${nodeLabel} ===`);
  console.log(`节点ID: ${clientId}`);

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
    const report: NodeReport = { node, nodeLabel, action };
    socket.emit('node:action', report);
  }

  async function handleMessage(msg: EmergencyMessage): Promise<void> {
    const rule = DISPATCH_RULES[msg.type];
    if (!rule) {
      return;
    }
    if (!rule.nodes.includes(node)) {
      return;
    }

    const actions = createActionsForMessage(msg, node, rule.actions);
    console.log(`[${nodeLabel}] 收到 #${msg.seq} ${msg.type} @ ${msg.location} → ${actions.length} 个操作`);

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

  await consumeLoop(clientId, handleMessage, abortController.signal);
}

main().catch((err) => {
  console.error('Fatal error:', err);
  process.exit(1);
});
