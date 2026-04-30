# MQ 校园应急响应系统

基于 ActiveMQ 的分布式校园应急信号收发与监控系统（发布/订阅模式）。

## 架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         ActiveMQ Broker                             │
│                   https://mq.usst2.bobliu.tech                      │
│                   Topic: campus.emergency                           │
└──────────┬─────────────────────────────────┬────────────────────────┘
           │  POST (REST API)                │ GET long-polling (REST API)
           │                                 │
    ┌──────▼──────┐              ┌───────────┼───────────┐
    │   sender/   │              │           │           │
    │  (发送端)    │       ┌──────▼──┐  ┌─────▼───┐  ┌───▼──────┐
    │  TUI 界面   │       │ 闸机    │  │ 短信    │  │ 监控中心  │
    │  批量发送   │       │ 控制器   │  │ 发送器   │  │ Dashboard │
    │  到 Topic   │       │ 进程1    │  │ 进程2    │  │ + Web UI  │
    └─────────────┘       │ ...更多   │  │ ...更多   │  └─────┬─────┘
                          └────┬─────┘  └────┬─────┘        │
                               │             │              │
                               │  WebSocket  │              │
                               └─────────┬───┘              │
                                         │                  │
                                   Socket.io ───────────────┘
                                         │
                                  ┌──────▼──────┐
                                  │   Browser   │
                                  │  监控界面    │
                                  │ Petite-Vue  │
                                  └─────────────┘
```

### 发送端 (sender/)

- TypeScript + Node.js + blessed 终端界面
- 通过 ActiveMQ REST API 向 Topic 发布紧急事件消息
- 支持 start/stop/exit 命令控制发送
- 每次发送 50 条消息，间隔 1 秒

### 接收端 (receiver/) — 发布/订阅模式

- TypeScript + Node.js 多进程架构
- 每个进程通过 `--processor=` 参数指定处理器类型
- 所有处理器订阅同一 Topic `campus.emergency`，各自过滤负责的操作
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
- 实时显示消息流、操作状态、统计数据
- 支持按紧急类型筛选

## 消息格式

```json
{
  "type": "火灾",
  "location": "第一教学楼",
  "timestamp": 1714459200000,
  "seq": 42
}
```

| 字段      | 类型   | 说明                    |
| --------- | ------ | ----------------------- |
| type      | string | 紧急事件类型，共 13 种  |
| location  | string | 具体建筑名称，共 50+ 个 |
| timestamp | number | 毫秒时间戳              |
| seq       | number | 消息递增序号            |

## 应急事件分发规则

| 事件类型 | 负责节点       | 触发操作                       |
| -------- | -------------- | ------------------------------ |
| 火灾     | 保卫处         | 启动火灾警报 + 闸机全开        |
| 燃气泄漏 | 保卫处、医务室 | 切断电源 + 疏散短信            |
| 爆炸     | 保卫处、医务室 | 启动警报 + 全校短信 + 医疗调度 |
| 恐怖袭击 | 保卫处、医务室 | 启动警报 + 闸机锁死 + 全校短信 |
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
pnpm dashboard
```

打开浏览器访问 `http://localhost:3456`

### 3. 启动接收端处理器

分别在多个终端启动不同处理器：

```bash
cd receiver

# 闸机控制器
pnpm start --processor=gate-controller --user=YOUR_USERNAME --pass=YOUR_PASSWORD

# 短信发送器
pnpm start --processor=sms-sender --user=YOUR_USERNAME --pass=YOUR_PASSWORD

# 警报控制器
pnpm start --processor=alarm-controller --user=YOUR_USERNAME --pass=YOUR_PASSWORD

# 电源控制器
pnpm start --processor=power-controller --user=YOUR_USERNAME --pass=YOUR_PASSWORD

# 水阀控制器
pnpm start --processor=valve-controller --user=YOUR_USERNAME --pass=YOUR_PASSWORD

# 医疗调度器
pnpm start --processor=medical-dispatcher --user=YOUR_USERNAME --pass=YOUR_PASSWORD
```

每个处理器可以启动多个实例，Topic 模式下所有实例都会收到每条消息，各自过滤处理。

### 4. 启动发送端

```bash
cd sender
npx tsx index.ts
```

在 TUI 界面中输入 ActiveMQ 凭据登录后，输入 `start` 开始发送消息。

发送端命令：
- `start` — 开始批量发送（50条/秒）
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
        │   ├── types.ts    # 类型定义
        │   ├── constants.ts # 配置 + 分发规则
        │   └── consumer.ts  # ActiveMQ REST 消费者
        ├── handlers/
        │   └── executor.ts  # 应急操作执行器
        ├── node/
        │   └── main.ts      # 接收端节点主进程
        └── dashboard/
            └── server.ts    # Dashboard 服务端
```
