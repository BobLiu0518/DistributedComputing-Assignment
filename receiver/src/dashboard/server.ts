import express from 'express';
import { createServer } from 'node:http';
import { Server } from 'socket.io';
import { DASHBOARD_PORT, MONITOR_INTERVAL_MS } from '../shared/constants.js';
import { setAuth } from '../shared/consumer.js';
import { fetchQueueAlerts } from '../shared/monitor.js';
import type { NodeReport, QueueAlert } from '../shared/types.js';

function parseAuthArgs(): { username?: string; password?: string } {
  const args = process.argv.slice(2);
  const userArg = args.find((a) => a.startsWith('--user='))?.split('=')[1];
  const passArg =
    args.find((a) => a.startsWith('--password='))?.split('=')[1] ??
    args.find((a) => a.startsWith('--pass='))?.split('=')[1];
  return { username: userArg, password: passArg };
}

const app = express();
const httpServer = createServer(app);
const io = new Server(httpServer, {
  cors: { origin: '*' },
});

app.use(express.static('public'));

const reports: NodeReport[] = [];
const MAX_REPORTS = 500;
const queueAlerts: QueueAlert[] = [];

let monitorEnabled = false;

function startMonitor(): void {
  if (monitorEnabled) return;
  monitorEnabled = true;

  async function poll(): Promise<void> {
    const alerts = await fetchQueueAlerts();
    if (alerts.length > 0) {
      for (const a of alerts) {
        const idx = queueAlerts.findIndex((q) => q.processor === a.processor);
        if (idx >= 0) {
          queueAlerts[idx] = a;
        } else {
          queueAlerts.push(a);
        }
      }

      const critical = alerts.filter((a) => a.level === 'critical');
      if (critical.length > 0) {
        console.log(`[monitor] CRITICAL: ${critical.map((a) => a.processor).join(', ')}`);
      }
    }

    io.emit('monitor:queues', queueAlerts);
  }

  poll();
  setInterval(poll, MONITOR_INTERVAL_MS);
  console.log(`[monitor] 已启动（间隔 ${MONITOR_INTERVAL_MS / 1000}s）`);
}

io.on('connection', (socket) => {
  console.log(`[dashboard] 客户端连接: ${socket.id}`);
  socket.emit('reports:init', reports);
  socket.emit('monitor:queues', queueAlerts);
  socket.emit('monitor:active', monitorEnabled);

  socket.on('node:action', (report: NodeReport) => {
    reports.unshift(report);
    if (reports.length > MAX_REPORTS) {
      reports.length = MAX_REPORTS;
    }
    io.emit('report:new', report);
  });

  socket.on('disconnect', (reason) => {
    console.log(`[dashboard] 客户端断开: ${socket.id} (${reason})`);
  });
});

app.get('/api/reports', (_req, res) => {
  res.json(reports);
});

app.get('/api/monitor', (_req, res) => {
  res.json(queueAlerts);
});

httpServer.listen(DASHBOARD_PORT, () => {
  console.log(`\n=== MQ 监控中心 ===`);
  console.log(`Dashboard: http://localhost:${DASHBOARD_PORT}`);

  const { username, password } = parseAuthArgs();
  if (username && password) {
    setAuth(username, password);
    startMonitor();
  } else {
    console.log(`[monitor] 未提供凭证（--user --password），堆积监控未启动`);
    console.log(`用法: pnpm dashboard --user=xxx --password=xxx`);
  }

  console.log(`等待接收端节点连接...\n`);
});
