# 简易 RPC 框架 — 架构设计

## 1. 概述

### 1.1 目标

实现一个跨语言的 RPC 框架，使 Java/Kotlin 调用端能够像调用本地函数一样透明地调用远程 Go 服务端上的函数。

### 1.2 技术选型

| 组件 | 语言 | 核心理由 |
|------|------|----------|
| **注册中心** | Rust | 学习 Rust 的切入点；逻辑简单，可专注语言学习；tokio + prost 生态成熟 |
| **服务端** | Go | goroutine 并发模型优雅；标准库网络编程能力足够；学习成本低于 Rust |
| **调用端（框架 + 业务）** | Kotlin + Java | 作业延续；注解 + 动态代理实现透明调用；Netty 简化 TCP 编程 |
| **序列化** | Protobuf | 二进制、跨语言、高性能、类型安全 |
| **传输** | TCP 长连接 | 避免 HTTP 协议开销；Netty 处理粘包/拆包 |

### 1.3 约束

- 不使用 HTTP（太重）
- 不使用 JSON（牺牲性能）
- Kotlin 代码与 Java 完全兼容（不使用 `suspend`、`sealed class` 等 Java 不兼容特性）
- 注册中心单机部署（学习项目，无需高可用）

---

## 2. 整体架构

```
                    ┌──────────────┐
                    │  注册中心     │  (Rust)
                    │  - 服务注册   │
                    │  - 心跳检测   │
                    │  - 服务发现   │
                    │  - 变更推送   │
                    └──┬────────┬──┘
             ②注册+心跳│        │③查询+订阅
                      │        │
    ┌─────────────────┴─┐  ┌──┴─────────────────┐
    │   服务端 (Go)      │  │  调用端 (Kotlin/Java) │
    │  - 业务逻辑实现    │  │  - RPC 框架          │
    │  - RPC 框架        │  │  - 业务调用代码      │
    └────────────────────┘  └──────────────────────┘
               ▲                       │
               └─── ④RPC 调用 ─────────┘
                   (TCP 直连)
```

**通信关系：**

| 通道 | 方向 | 协议 | 连接方式 |
|------|------|------|----------|
| ① 注册中心 ↔ 服务端 | 双向 | 自定义 Protobuf | TCP 长连接 |
| ② 注册中心 ↔ 调用端 | 双向（推送） | 自定义 Protobuf | TCP 长连接 |
| ③ 调用端 ↔ 服务端 | 请求-响应 | 自定义 Protobuf | TCP 长连接（连接池） |

---

## 3. Protobuf 定义

分为两层：**框架层**（RPC 框架作者维护）和**业务层**（框架使用者维护）。

### 3.1 框架层 — RPC 调用协议 (`proto/rpc.proto`)

```protobuf
syntax = "proto3";
package rpc;

// 调用请求
message RpcRequest {
  string request_id = 1;  // 请求唯一ID，用于请求-响应匹配
  string service    = 2;  // 服务名，如 "UserService"
  string method     = 3;  // 方法名，如 "getUser"
  bytes  payload    = 4;  // 业务参数序列化后的字节（业务 Protobuf 消息）
}

// 调用响应
message RpcResponse {
  string request_id = 1;  // 对应请求的ID
  bytes  payload    = 2;  // 业务返回值序列化（成功时）
  Error  error      = 3;  // 错误信息（失败时）
}

// 错误信息
message Error {
  int32  code    = 1;  // 错误码
  string message = 2;  // 错误描述
}
```

### 3.2 框架层 — 注册中心协议 (`proto/registry.proto`)

```protobuf
syntax = "proto3";
package registry;

// --- 服务端 → 注册中心 ---

// 服务注册
message RegisterRequest {
  string ip   = 1;
  int32  port = 2;
  string service = 3;  // 该服务端支持的单个服务名
}

// 心跳
message HeartbeatRequest {
  string ip   = 1;
  int32  port = 2;
}

// --- 注册中心 → 服务端 / 调用端 ---

// 通用响应
message RegistryResponse {
  bool   success = 1;
  string message = 2;
}

// --- 注册中心 → 调用端（推送） ---

// 服务信息
message ServiceInfo {
  string ip   = 1;
  int32  port = 2;
}

// 服务列表推送（全量）
message ServiceList {
  map<string, ServiceListEntry> services = 1;
}

message ServiceListEntry {
  repeated ServiceInfo instances = 1;
}
```

### 3.3 业务层 — 示例 (`proto/example/user_service.proto`)

```protobuf
syntax = "proto3";
package example;

message GetUserRequest {
  int64 id = 1;
}

message GetUserResponse {
  int64  id   = 1;
  string name = 2;
  int32  age  = 3;
}
```

**说明：** 业务 Protobuf 的 Request/Response 的序列化字节，作为框架层 `RpcRequest.payload` / `RpcResponse.payload` 的值传输。两层嵌套：框架层负责路由和错误传递，业务层负责数据语义。

---

## 4. 模块划分

### 4.1 项目目录结构

```
RPC/
├── proto/                        # 共享 Protobuf 定义
│   ├── rpc.proto                 #   框架层：RPC 调用协议
│   ├── registry.proto            #   框架层：注册中心协议
│   └── example/                  #   业务层：示例（非框架代码）
│       └── user_service.proto
│
├── registry/                     # 注册中心 (Rust)
│   ├── Cargo.toml
│   └── src/
│       ├── main.rs               #   入口
│       ├── config.rs             #   配置（地址、心跳间隔、超时）
│       ├── network/
│       │   ├── mod.rs
│       │   ├── server.rs         #   TCP 服务端：监听、接受连接
│       │   └── codec.rs          #   帧编解码（基于长度前缀）
│       ├── registry/
│       │   ├── mod.rs
│       │   ├── store.rs          #   服务存储（内存 HashMap）：增删查
│       │   ├── health.rs         #   健康检查：心跳时间戳、过期检测
│       │   └── notifier.rs       #   变更推送：通知所有连接的调用端
│       └── proto/                #   prost 编译生成的 Rust 代码
│
├── server/                       # 服务端 (Go)
│   ├── go.mod
│   ├── cmd/main.go               #   入口
│   ├── config/config.go          #   配置
│   ├── network/
│   │   ├── server.go             #   TCP 服务端：监听、接受连接
│   │   └── codec.go              #   帧编解码
│   ├── registry/
│   │   └── client.go             #   注册中心客户端：注册、心跳
│   ├── router/
│   │   └── router.go             #   方法路由：service.method → handler
│   ├── handler/
│   │   └── handler.go            #   通用处理：反序列化 → 调用 → 序列化 → 响应
│   └── proto/                    #   protoc 编译生成的 Go 代码
│
├── client/                       # 调用端 (Kotlin/Java)
│   ├── build.gradle.kts
│   └── src/main/kotlin/tech/bobliu/rpc/
│       ├── annotation/
│       │   ├── RpcService.kt     #   @RpcService 注解
│       │   └── RpcMethod.kt      #   @RpcMethod 注解
│       ├── scanner/
│       │   └── ClassScanner.kt   #   类路径扫描（已有基础版本）
│       ├── proxy/
│       │   └── RpcProxyFactory.kt #  动态代理生成（JDK Proxy）
│       ├── serialize/
│       │   └── ProtobufBridge.kt #  序列化桥接：方法注解 → Protobuf 构造
│       ├── network/
│       │   ├── RpcClient.kt      #   TCP 客户端（Netty）：连接池、编码/解码
│       │   └── ConnectionPool.kt #   连接池管理
│       ├── registry/
│       │   └── RegistryClient.kt #   注册中心客户端：发现服务、订阅更新
│       ├── balance/
│       │   └── LoadBalancer.kt   #   负载均衡：随机选择
│       ├── fault/
│       │   └── FaultTolerance.kt #   容错：指数退避、失败重试
│       └── exception/
│           └── RpcException.kt   #   RPC 异常
│
└── docs/
    └── architecture.md           # 本文档
```

### 4.2 注册中心模块 (Rust)

| 模块 | 文件 | 职责 |
|------|------|------|
| **入口** | `main.rs` | 加载配置，启动 TCP 服务端，启动健康检查定时器 |
| **配置** | `config.rs` | 监听地址、心跳间隔（建议 10s）、心跳超时（建议 30s） |
| **网络-服务端** | `network/server.rs` | `TcpListener` 监听，为每个连接 `tokio::spawn` 独立处理任务 |
| **网络-编解码** | `network/codec.rs` | 实现 `tokio_util::codec::Encoder/Decoder`，基于长度前缀的帧协议：`[4字节长度][protobuf 消息体]` |
| **服务存储** | `registry/store.rs` | `HashMap<String, Vec<ServiceInstance>>`，并发安全用 `Arc<RwLock<>>` |
| **健康检查** | `registry/health.rs` | 存储心跳时间戳，定时扫描过期服务，触发注销和推送 |
| **变更推送** | `registry/notifier.rs` | 维护已连接的调用端列表，变更时将全量服务列表推送给所有调用端 |

**关键设计决策：**

- 存储结构：`serviceName → [(ip, port, lastHeartbeat), ...]`
- 因为每个服务端只提供一个服务（简化设计），服务端地址 = (ip, port) 作为唯一连接标识
- 同一个服务名下可以有多个实例（扩展性考虑）
- 调用端列表单独维护，用于推送通知

### 4.3 服务端模块 (Go)

| 模块 | 文件 | 职责 |
|------|------|------|
| **入口** | `cmd/main.go` | 加载配置，连接注册中心，注册服务，启动 RPC 监听 |
| **配置** | `config/config.go` | RPC 监听地址、注册中心地址、注册的服务名 |
| **网络-服务端** | `network/server.go` | `net.Listen` 监听，每个连接 goroutine 处理 |
| **网络-编解码** | `network/codec.go` | 帧读写：`[4字节长度][protobuf 消息体]` |
| **注册客户端** | `registry/client.go` | 连接注册中心，发送注册请求，启动定时心跳 goroutine |
| **方法路由** | `router/router.go` | `map[string]HandlerFunc`，key 为 `"service.method"` |
| **通用处理** | `handler/handler.go` | 接收 `RpcRequest` → 路由到处理函数 → 构造 `RpcResponse`（含错误） |

**关键设计决策：**

- 每个服务端进程只注册一个 service（但可以有多个 method）
- Handler 的签名：`func(ctx context.Context, payload []byte) ([]byte, error)`
  - 业务层负责将 `payload` 反序列化为具体类型
  - 框架层只负责路由和错误封装
- goroutine 模型：每个 TCP 连接一个读循环 goroutine

### 4.4 调用端模块 (Kotlin/Java)

| 模块 | 文件 | 职责 |
|------|------|------|
| **注解** | `annotation/RpcService.kt` | 标记接口为 RPC 服务，`value` 指定服务名 |
| **注解** | `annotation/RpcMethod.kt` | 标记方法，`requestType` / `responseType` 指定 Protobuf 类 |
| **扫描器** | `scanner/ClassScanner.kt` | 扫描类路径，找出所有 `@RpcService` 接口 |
| **代理工厂** | `proxy/RpcProxyFactory.kt` | 为每个接口创建 `java.lang.reflect.Proxy`，拦截方法调用 |
| **序列化桥接** | `serialize/ProtobufBridge.kt` | 从 `@RpcMethod` 注解获取 Protobuf 类 → 反射调用 `parseFrom()` / `toByteArray()` |
| **网络客户端** | `network/RpcClient.kt` | Netty `Bootstrap`，封装连接、发送、接收 |
| **连接池** | `network/ConnectionPool.kt` | `Map<address, Channel>`，惰性创建，心跳保活 |
| **注册客户端** | `registry/RegistryClient.kt` | 连接注册中心，获取服务列表，订阅变更推送 |
| **负载均衡** | `balance/LoadBalancer.kt` | 从健康实例中随机选取 |
| **容错** | `fault/FaultTolerance.kt` | 指数退避重试（1s → 2s → 4s …），超过最大次数抛异常 |
| **异常** | `exception/RpcException.kt` | 封装 `Error.code` 和 `Error.message` |

**关键设计决策：**

- 动态代理使用 JDK 原生 `java.lang.reflect.Proxy`（只代理接口，不需要 CGLIB）
- `@RpcMethod` 必须携带 `requestType` 和 `responseType`，框架据此反射调用 Protobuf 序列化方法
- 发现流程：首次调用某服务 → 查本地缓存 → 缓存未命中则向注册中心查询 → 建立连接 → 缓存
- 故障转移：调用失败 → 标记该实例为不可用 → 从注册中心获取最新列表 → 重试另一个实例

---

## 5. 数据流

### 5.1 服务注册与发现流程

```
时间线

服务端                    注册中心                   调用端
  │                         │                         │
  │──① TCP 连接 ──────────→│                         │
  │──② RegisterRequest ───→│                         │
  │                         │  存储服务信息            │
  │                         │──③ RegistryResponse ──→│
  │                         │                         │
  │──④ 定时心跳 ───────────→│                         │
  │  (每 10s)               │  更新心跳时间戳          │
  │                         │                         │
  │                         │          ⑤ TCP 连接 ──→│
  │                         │←──⑥ ServiceList ──────│ (全量推送)
  │                         │                         │
  │  (心跳超时，被踢出)     │                         │
  │                         │──⑦ ServiceList ───────→│ (更新：该服务移除)
  │                         │                         │
```

### 5.2 RPC 调用流程

```
调用端代码: userService.getUser(req)
                │
                ▼
┌───────────────────────────────┐
│ 1. 动态代理拦截              │
│    获取方法上的 @RpcMethod   │
│    service = "UserService"   │
│    method  = "getUser"       │
└───────────────┬───────────────┘
                ▼
┌───────────────────────────────┐
│ 2. 序列化参数                │
│    req.toByteArray() → bytes │
└───────────────┬───────────────┘
                ▼
┌───────────────────────────────┐
│ 3. 构造 RpcRequest           │
│    request_id = UUID          │
│    service    = "UserService" │
│    method     = "getUser"     │
│    payload    = [bytes]       │
└───────────────┬───────────────┘
                ▼
┌───────────────────────────────┐
│ 4. 负载均衡                  │
│    查本地缓存找出 UserService│
│    的实例列表 → 随机选一个   │
│    e.g. 192.168.1.5:8080     │
└───────────────┬───────────────┘
                ▼
┌───────────────────────────────┐
│ 5. 从连接池获取/创建连接     │
│    发送 RpcRequest (二进制)  │
└───────────────┬───────────────┘
                │  TCP ──────────────────→ 服务端
                │                              │
                │              ┌───────────────┴───────────────┐
                │              │ 6. 解析 RpcRequest            │
                │              │ 7. 路由: "UserService.getUser"│
                │              │ 8. 反序列化 payload           │
                │              │ 9. 调用 handler(req)          │
                │              │10. 序列化返回值                │
                │              │11. 构造 RpcResponse           │
                │              └───────────────┬───────────────┘
                │  TCP ←──────────────────    │
                ▼
┌───────────────────────────────┐
│12. 解析 RpcResponse          │
│    检查 error 字段            │
│    - 有错误 → 抛 RpcException │
│    - 成功   → 反序列化 payload│
│                              │
│13. 返回 GetUserResponse      │
│    调用端代码拿到结果         │
└───────────────────────────────┘
```

### 5.3 错误传递流程

```
服务端 handler 执行出错
        │
        ▼
handler 返回 error
        │
        ▼
框架捕获 error，构造 RpcResponse {
    error: Error {
        code: 500,
        message: "user not found: id=999"
    }
}
        │
  TCP ──→ 调用端
        │
        ▼
调用端解析 RpcResponse.error != null
        │
        ▼
throw RpcException(code=500, message="user not found: id=999")
        │
        ▼
调用端代码 try/catch 处理
```

---

## 6. 连接管理与容错

### 6.1 连接拓扑

```
注册中心 (端口 9000)
  ├── 服务端连接 A (192.168.1.5:45678)
  ├── 服务端连接 B (192.168.1.6:45678)
  ├── 调用端连接 C (192.168.1.10:52341)
  └── 调用端连接 D (192.168.1.11:52341)

调用端 C 的连接池
  ├── → 服务端 A:8080 (UserService)
  ├── → 服务端 B:8080 (OrderService)
  └── → 服务端 C:8080 (UserService)  [备用]
```

### 6.2 容错策略

| 场景 | 策略 |
|------|------|
| 调用端连接注册中心失败 | 启动失败，直接报错退出（注册中心是基础依赖） |
| 调用端与注册中心断连 | 使用本地缓存的服务列表继续工作；后台重连注册中心 |
| 调用端连接服务端失败 | 尝试同一服务的其他实例；都失败则向注册中心查询更新列表 |
| RPC 调用超时 | 重试（指数退避：1s → 2s → 4s）；超过 3 次抛出 RpcException |
| 服务端实例全部不可用 | 抛出 RpcException，提示服务不可用 |
| 服务端与注册中心断连 | 注册中心心跳超时后将其移除，推送更新给所有调用端 |

### 6.3 帧协议

所有 TCP 连接使用统一帧格式：

```
┌──────────────────┬────────────────────────────┐
│  4 字节 (大端)    │  变长                       │
│  消息体长度       │  Protobuf 序列化后的消息体   │
└──────────────────┴────────────────────────────┘
```

这是 TCP 流式传输下的标准做法。接收方先读 4 字节确定消息长度，再读对应长度的字节，解决粘包/拆包问题。

- Java (Netty)：使用 `LengthFieldBasedFrameDecoder` + `ProtobufDecoder`
- Go：自实现 `codec.go`（约 30 行）
- Rust：实现 `tokio_util::codec::Decoder` trait（约 30 行）

---

## 7. 注解设计 (Java/Kotlin)

```kotlin
// 标记一个接口为 RPC 服务
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class RpcService(
    val value: String = ""  // 服务名，默认取接口名（如 "UserService"）
)

// 标记一个方法为 RPC 方法
@Target(AnnotationTarget.FUNCTION)
@Retention(AnnotationRetention.RUNTIME)
annotation class RpcMethod(
    val requestType: KClass<*>,   // 请求 Protobuf 类（如 GetUserRequest::class）
    val responseType: KClass<*>   // 响应 Protobuf 类（如 GetUserResponse::class）
)
```

**开发者使用示例：**

```kotlin
@RpcService("UserService")
interface UserService {
    @RpcMethod(
        requestType = GetUserRequest::class,
        responseType = GetUserResponse::class
    )
    fun getUser(request: GetUserRequest): GetUserResponse
}

// 调用
val userService: UserService = RpcFramework.createProxy(UserService::class.java)
val response = userService.getUser(GetUserRequest.newBuilder().setId(123).build())
```

---

## 8. 待定 / 可扩展项

这些是在当前学习项目范围内不做、但预留了扩展空间的点：

- [ ] **增量推送**：注册中心目前全量推送，可在 `ServiceList` 中加 `version` 字段做增量
- [ ] **负载均衡策略**：目前随机选择，可扩展为轮询、最少连接数、加权等
- [ ] **服务端多 service**：目前每进程一个 service，可扩展为一个进程注册多个 service
- [ ] **注册中心持久化**：目前纯内存，可扩展为定期快照到文件
- [ ] **调用端本地缓存失效策略**：目前依赖注册中心推送更新
- [ ] **调用链超时传递**：`RpcRequest` 可携带 deadline 信息
- [ ] **服务版本管理**：服务注册时可携带版本号
