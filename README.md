# MQ 校园应急响应系统

基于 ActiveMQ Virtual Topic 的分布式校园应急信号收发与监控系统（发布/订阅 + 持久消费）。

## 架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         ActiveMQ Broker                             │
│                   https://mq.usst2.bobliu.tech                      │
│           VirtualTopic.campus.emergency                             │
│                                                                     │
│  ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐     │
│  │ Consumer.gate-   │ │ Consumer.sms-    │ │ Consumer.dash-   │     │
│  │ controller.      │ │ sender.          │ │ board.           │     │
│  │ VirtualTopic...  │ │ VirtualTopic...  │ │ VirtualTopic...  │     │
│  │   (持久 Queue)    │ │   (持久 Queue)    │ │   (持久 Queue)    │     │
│  └────────┬─────────┘ └────────┬─────────┘ └────────┬─────────┘     │
│           │                    │                    │               │
│  Dashboard ──── Jolokia JMX ───┤ (每10s查堆积)      │               │
└───────────┼────────────────────┼────────────────────┼───────────────┘
            │ STOMP/WebSocket    │                    │ STOMP/WebSocket
     ┌──────▼──┐          ┌──────▼──┐          ┌──────▼──────────┐
     │ 闸机    │          │ 短信    │          │ Dashboard 自订阅 │
     │ 控制器   │          │ 发送器   │          │ (消息流数据源)    │
     │ 实例N   │          │ 实例N   │          └──────┬───────────┘
     └────┬────┘          └────┬────┘                │
          │                    │    Socket.io        │ Socket.io
          └────────────────────┼─────────────────────┘
                               │
                         Socket.io ─────────── Web UI (Petite-Vue)
```

**Virtual Topic 语义：** 生产者发到 `VirtualTopic.campus.emergency`，每个消费者组自动创建独立的持久 Queue `Consumer.{name}.VirtualTopic...`。同组内多实例共享一个 Queue（竞争消费），消费者离线后消息持久堆积不丢失。

**STOMP over WebSocket：** 各处理器通过 STOMP over WebSocket 长连接消费消息。WebSocket 断开时订阅自动清理，无僵尸 consumer 残留。Nginx 代理 `wss://mq.usst2.bobliu.tech/ws/` → ActiveMQ WebSocket transport (61614)。

### 发送端 (sender/)

- TypeScript + Node.js + blessed 终端界面
- 支持 CLI 传参跳过登录：`--user=xxx --password=xxx`
- 通过 ActiveMQ REST API 向 VirtualTopic 发布紧急事件消息
- 命令：`start`（连续发送）、`send N`（一次发送 N 条，默认 50）、`stop`、`exit`

### 接收端 (receiver/) — STOMP 消费模式

- TypeScript + Node.js 多进程架构
- 每个进程通过 `--processor=` 参数指定处理器类型
- 各处理器通过 **STOMP over WebSocket** 从独立持久 Queue `Consumer.{processor}.VirtualTopic.campus.emergency` 消费
- STOMP 长连接，断开自动清理订阅
- 同处理器可启动多个实例，共享一个 Queue（竞争消费，负载均衡）
- 处理器离线期间消息持久堆积，恢复后继续消费
- 应急操作为模拟耗时任务，延迟为基准值 × (1.0 ~ 1.5) 随机浮动
- 操作状态通过 Socket.io 实时上报 Dashboard

#### 处理器类型与职责

| 处理器             | 负责操作                                       |
| ------------------ | ---------------------------------------------- |
| gate-controller    | gate_open（闸机全开）、gate_lock（闸机锁死）   |
| sms-sender         | sms_all、sms_security、sms_medical（短信通知） |
| alarm-controller   | alarm（启动警报）                              |
| power-controller   | power_cut（切断电源）                          |
| valve-controller   | valve_close（关闭水阀）                        |
| medical-dispatcher | medical_dispatch（医疗调度）                   |

### 监控界面

- Express + Socket.io 提供 WebSocket 服务
- Petite-Vue 驱动的纯前端卡片式面板
- **6 处理器队列卡片**（3×2 网格），绿/黄/红 实时状态 + 脉冲告警动画
- 严重堆积时显示红色告警横幅
- 实时显示消息流、操作状态、统计数据
- 支持按紧急类型筛选
- **Dashboard 自订阅**：Dashboard 启动时通过 STOMP 消费自己的 VirtualTopic 队列 `Consumer.dashboard.VirtualTopic.campus.emergency`，获取全量消息流（含被 skip 的消息）
- **JMX 指标**：`接收消息`（EnqueueCount）和队列卡片数据（QueueSize、ConsumerCount）均通过 Jolokia JMX 从 Broker 直接读取，不依赖处理器上报

## 消息格式

```json
{
  "type": "火灾",
  "location": "第一教学楼",
  "timestamp": 1714459200000,
  "seq": 42,
  "msgid": "a1b2c3d4-e5f6-..."
}
```

| 字段      | 类型   | 说明                    |
| --------- | ------ | ----------------------- |
| type      | string | 紧急事件类型，共 13 种  |
| location  | string | 具体建筑名称，共 50+ 个 |
| timestamp | number | 毫秒时间戳              |
| seq       | number | 消息递增序号（客户端，重启归零） |
| msgid     | string | UUID，全局唯一标识，用于处理器去重 |

## 应急事件分发规则

| 事件类型 | 负责节点       | 触发操作                       |
| -------- | -------------- | ------------------------------ |
| 火灾     | 保卫处         | 启动火灾警报 + 闸机全开        |
| 燃气泄漏 | 保卫处、医务室 | 切断电源 + 疏散短信            |
| 爆炸     | 保卫处、医务室 | 启动警报 + 全校短信 + 医疗调度 |
| 恐怖袭击 | 保卫处、医务室 | 启动警报 + 闸机全开 + 全校短信 |
| 水管爆裂 | 保卫处         | 关闭水阀                       |
| 非法入侵 | 保卫处         | 启动警报 + 闸机锁死 + 通知保安 |
| 人员伤亡 | 医务室         | 医疗调度 + 通知医务室          |
| 生化泄漏 | 保卫处、医务室 | 启动警报 + 切断电源 + 疏散短信 |
| 打架斗殴 | 保卫处         | 通知保安                       |
| 食物中毒 | 医务室         | 通知医务室 + 医疗调度          |
| 电梯困人 | 保卫处         | 通知保安                       |
| 踩踏事件 | 保卫处、医务室 | 启动警报 + 医疗调度 + 疏散短信 |

## 运行方式

### 环境要求

- Node.js >= 18
- pnpm >= 10

### 1. 安装依赖

```bash
cd sender && pnpm install
cd ../receiver && pnpm install
```

### 2. 启动 Dashboard（监控中心）

```bash
cd receiver

# 基础模式（无堆积监控、无消息流）
pnpm dashboard

# 完整模式（需 ActiveMQ 凭据）
pnpm dashboard --user=YOUR_USERNAME --pass=YOUR_PASSWORD
```

打开浏览器访问 `http://localhost:3456`

Dashboard 在完整模式下同时启动：
- **Jolokia JMX 监控**（每 10s）：查询各处理器队列的 QueueSize、ConsumerCount、EnqueueCount
- **STOMP 自订阅**：消费 `Consumer.dashboard.VirtualTopic.campus.emergency`，作为消息流数据源

### 3. 启动接收端处理器

分别在多个终端启动不同处理器：

```bash
cd receiver
pnpm start --processor=gate-controller --user=YOUR_USERNAME --pass=YOUR_PASSWORD
pnpm start --processor=sms-sender --user=YOUR_USERNAME --pass=YOUR_PASSWORD
pnpm start --processor=alarm-controller --user=YOUR_USERNAME --pass=YOUR_PASSWORD
pnpm start --processor=power-controller --user=YOUR_USERNAME --pass=YOUR_PASSWORD
pnpm start --processor=valve-controller --user=YOUR_USERNAME --pass=YOUR_PASSWORD
pnpm start --processor=medical-dispatcher --user=YOUR_USERNAME --pass=YOUR_PASSWORD
```

每个处理器通过 STOMP over WebSocket 消费独立持久队列，同类型多实例共享同一队列（竞争消费）。每个操作有 5% 模拟失败率，3 次重试（指数退避 1s→2s→4s）。

### 4. 启动发送端

```bash
cd sender
npx tsx index.ts --user=YOUR_USERNAME --password=YOUR_PASSWORD
```

发送端命令：
- `start` — 连续发送（50条/秒）
- `send N` — 一次发送 N 条（不填默认 50）
- `stop` — 停止发送
- `exit` — 退出

## 项目结构

```
MQ/
├── data/
│   ├── emergencies.json    # 紧急事件类型定义
│   └── locations.json      # 校园建筑位置数据
├── sender/                 # 发送端
│   ├── index.ts            # 主程序（TUI 界面 + 消息发送）
│   ├── package.json
│   └── tsconfig.json
└── receiver/               # 接收端
    ├── package.json
    ├── tsconfig.json
    ├── public/
    │   └── index.html      # Web 监控界面
    └── src/
        ├── shared/
        │   ├── types.ts          # 类型定义
        │   ├── constants.ts      # 配置 + 分发规则 + 处理器定义
        │   ├── auth.ts           # ActiveMQ 认证
        │   ├── stomp-consumer.ts # STOMP over WebSocket 消费者
        │   └── monitor.ts        # Jolokia JMX 队列监控
        ├── handlers/
        │   └── executor.ts       # 应急操作执行器（3次重试 + 退避）
        ├── node/
        │   └── main.ts           # 处理器主进程
        └── dashboard/
            └── server.ts         # Dashboard + Socket.io + Dashboard STOMP 自订阅
```

## Dashboard 指标说明

| 指标 | 数据来源 | 含义 |
|------|----------|------|
| 消息总数 | JMX EnqueueCount | Topic 自始以来的总消息数 |
| 接收消息 | JMX EnqueueCount | 同上 |
| 触发操作 | Socket.io | 去重后的操作数（按 actionId） |
| 执行中 | Socket.io | 当前 running 状态的操作 |
| 操作失败 | Socket.io | 最终 error 状态的操作 |
| 消息流 | Dashboard STOMP 自订阅 | 全量消息滚动（含 skip 的） |
| 队列卡片 | JMX (QueueSize, ConsumerCount) | 每 10s 更新 |
| 堆积告警 | JMX QueueSize | ≥500 黄，≥2000 或无消费者 红 |
