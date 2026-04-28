# 简易 RPC 框架

跨语言的 RPC 框架，使 Java 调用端能够像调用本地函数一样透明地调用远程服务。

## 架构

```
Java 调用端 ──RPC 调用──→ Go 服务端
    │                        │
    └──── 服务发现 ────→ Rust 注册中心 ←── 注册 + 心跳 ──┘
```

| 组件                  | 语言        | 职责                                   |
| --------------------- | ----------- | -------------------------------------- |
| [注册中心](registry/) | Rust        | 服务注册、心跳检测、服务发现、变更推送 |
| [服务端](server/)     | Go          | 处理 RPC 请求、执行业务逻辑            |
| [调用端](client/)     | Kotlin/Java | 注解驱动、动态代理、透明 RPC 调用      |

> 详细设计见 [docs/architecture.md](docs/architecture.md)

## 特性

- **透明调用**：`userService.getUser(req)` —— 看起来是本地方法，实际走 TCP 远程调用
- **跨语言**：Protobuf 序列化，Java ↔ Go ↔ Rust 互通
- **注解驱动**：`@RpcService` / `@RpcMethod` 声明接口，无需手写网络代码
- **动态代理**：JDK Proxy 拦截方法调用，自动完成序列化、网络传输、反序列化
- **服务发现**：调用端自动从注册中心获取服务地址，服务端上下线实时推送
- **负载均衡**：随机选取健康实例
- **容错**：指数退避重试（200ms → 400ms → 800ms → …）

## 快速开始

### 环境要求

- JDK 25 + Maven 3.9+
- Go 1.25+
- Rust 1.93+
- protoc 34+（Protobuf 编译器）

### 1. 启动注册中心

```bash
cd registry
cargo run
# 默认监听 0.0.0.0:9000
```

### 2. 启动服务端

```bash
cd server
go run ./cmd/server/
# 默认监听 0.0.0.0:8080，注册到 localhost:9000
```

### 3. 运行调用端

```bash
cd client
mvn exec:java
```

### 自定义配置

```bash
# 注册中心
REGISTRY_ADDR=0.0.0.0:9999 cargo run

# 服务端
RPC_PORT=9090 SERVICE_NAME=OrderService go run ./cmd/server/

# 调用端（修改 RpcClientDemo.java 中的 RpcConfig 参数）
```

## 开发者视角

框架使用者只需关注业务接口和 Protobuf 定义，网络细节完全透明：

```java
// 1. 定义接口
@RpcService("UserService")
public interface UserService {
    @RpcMethod(requestType = GetUserRequest.class, responseType = GetUserResponse.class)
    GetUserResponse getUser(GetUserRequest request);
}

// 2. 调用（就像本地方法）
RpcFramework.init(new RpcConfig("localhost", 9000));
UserService svc = RpcFramework.createProxy(UserService.class);
GetUserResponse resp = svc.getUser(GetUserRequest.newBuilder().setId(42).build());
```

## 项目结构

```
RPC/
├── proto/                  # 共享 Protobuf 定义
├── registry/               # 注册中心 (Rust)
├── server/                 # 服务端 (Go)
├── client/                 # 调用端 (Kotlin/Java)
└── docs/
    └── architecture.md     # 架构设计文档
```
