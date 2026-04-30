import express from 'express';
import { createServer } from 'node:http';
import { Server } from 'socket.io';
import { DASHBOARD_PORT } from '../shared/constants.js';
import type { NodeReport } from '../shared/types.js';

const app = express();
const httpServer = createServer(app);
const io = new Server(httpServer, {
  cors: { origin: '*' },
});

app.use(express.static('public'));

const reports: NodeReport[] = [];
const MAX_REPORTS = 500;

io.on('connection', (socket) => {
  console.log(`[dashboard] 客户端连接: ${socket.id}`);
  socket.emit('reports:init', reports);

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

httpServer.listen(DASHBOARD_PORT, () => {
  console.log(`\n=== MQ 监控中心 ===`);
  console.log(`Dashboard: http://localhost:${DASHBOARD_PORT}`);
  console.log(`等待接收端节点连接...\n`);
});
