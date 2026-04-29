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
- **代码生成**：Go 服务端从 proto `service` 定义自动生成强类型 Handler 接口和注册代码
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
- protoc-gen-go（`go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`）

### 1. 生成 Protobuf 代码

**服务端（Go）**

```bash
cd server
go generate ./...
```

**调用端（Java）**

```bash
cd client
mvn protobuf:compile
```

### 2. 启动注册中心

```bash
cd registry
cargo run
# 默认监听 0.0.0.0:9000
```

### 3. 启动服务端

```bash
cd server
go run ./cmd/server/
# 默认监听 0.0.0.0:8080，注册到 localhost:9000
```

### 4. 运行调用端

```bash
cd client
mvn exec:java
```

### 自定义配置

**注册中心（Rust）**

编辑 `registry/registry.toml`：

```toml
listen_addr = "0.0.0.0:9999"
heartbeat_timeout_secs = 30
health_check_interval_secs = 10
broadcast_channel_capacity = 16
```

或环境变量覆盖：

```bash
REGISTRY_ADDR=0.0.0.0:9999 cargo run
```

**服务端（Go）**

编辑 `server/server.json`：

```json
{
  "registry_addr": "localhost:9000",
  "port": 8080,
  "service_name": "UserService",
  "heartbeat_interval_sec": 10,
  "heartbeat_max_fail": 2,
  "reconnect_backoff_sec": 1,
  "reconnect_max_backoff_sec": 30
}
```

或环境变量覆盖：

```bash
RPC_PORT=9090 SERVICE_NAME=OrderService go run ./cmd/server/
```

**调用端（Java）**

修改 `@RpcApp` 注解中的 `registryHost` / `registryPort` 参数。

## 开发者指南

框架使用者只需关注业务逻辑，网络通信、序列化、服务发现全部由框架处理。

### 开发流程

```
1. 定义 Protobuf 消息 → 2. 生成代码 → 3. 实现业务逻辑 → 4. 运行
```

---

### 服务端（Go）

#### 1. 定义 Protobuf

在 `proto/` 下创建 `.proto` 文件，定义请求/响应消息和 service：

```protobuf
// proto/example/user_service.proto
syntax = "proto3";
package example;
option go_package = "rpc-server/pb/example";
option java_package = "tech.bobliu.rpc.proto.example";
option java_multiple_files = true;

message GetUserRequest {
  int64 id = 1;
}

message GetUserResponse {
  int64 id = 1;
  string name = 2;
  int32 age = 3;
}

service UserService {
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
}
```

#### 2. 生成代码

```bash
cd server
go generate ./...
```

这会生成两类代码：
- `pb/` — Protobuf 消息类型（`protoc` 生成）
- `internal/rpc/handlers_gen.go` — 强类型 Handler 接口 + 自动注册函数（`genrpc` 生成）

#### 3. 实现 Handler

实现生成的接口，只需写业务逻辑——序列化/反序列化由框架完成：

```go
// server/example/user_service.go
package example

import (
	"context"
	"fmt"

	pb "rpc-server/pb/example"
	"rpc-server/internal/rpc"
)

type UserService struct{}

func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	return &pb.GetUserResponse{
		Id:   req.Id,
		Name: fmt.Sprintf("user-%d", req.Id),
		Age:  25,
	}, nil
}

// 编译期检查：确保 UserService 实现了 UserServiceHandler 接口
var _ rpc.UserServiceHandler = (*UserService)(nil)
```

#### 4. 注册并启动

```go
// server/cmd/server/main.go
func main() {
	cfg := config.Load()
	r := router.New()

	// 一行注册，所有方法自动绑定
	rpc.RegisterUserService(r, &example.UserService{})

	// ... 其余启动逻辑不变
}
```

> **约定**：proto service 名 `UserService` 与 `server.json` 中 `service_name` 一致。Method 名自动转换为 camelCase（`GetUser` → `getUser`）以匹配调用端。

---

### 调用端（Java）

#### 1. 定义接口

```java
// client/src/main/java/tech/bobliu/rpc/app/UserService.java
package tech.bobliu.rpc.app;

import tech.bobliu.rpc.annotation.RpcMethod;
import tech.bobliu.rpc.annotation.RpcService;
import tech.bobliu.rpc.proto.example.GetUserRequest;
import tech.bobliu.rpc.proto.example.GetUserResponse;

@RpcService("UserService")
public interface UserService {
    @RpcMethod(requestType = GetUserRequest.class, responseType = GetUserResponse.class)
    GetUserResponse getUser(GetUserRequest request);
}
```

#### 2. 生成代码

```bash
cd client
mvn protobuf:compile
```

#### 3. 调用

```java
// client/src/main/java/tech/bobliu/rpc/app/RpcClientDemo.java
package tech.bobliu.rpc.app;

import tech.bobliu.rpc.annotation.RpcApp;
import tech.bobliu.rpc.annotation.RpcInject;
import tech.bobliu.rpc.proto.example.GetUserRequest;
import tech.bobliu.rpc.proto.example.GetUserResponse;

@RpcApp(basePackage = "tech.bobliu.rpc.app", registryHost = "localhost")
public class RpcClientDemo {
    @RpcInject
    private UserService userService;

    public void run() {
        GetUserRequest request = GetUserRequest.newBuilder().setId(42).build();
        GetUserResponse response = userService.getUser(request);
        System.out.printf("User: id=%d, name=%s, age=%d%n",
                response.getId(), response.getName(), response.getAge());
    }
}
```

---

### 规范

| 规则 | 说明 |
|------|------|
| **Proto 先行** | 请求/响应消息必须在 `proto/` 中定义，双端共享 |
| **service 定义** | 每个 proto 文件应包含 `service` 声明，用于驱动 Go 端代码生成 |
| **命名一致** | `service_name` 在 proto、`server.json`、`@RpcService` 中保持一致 |
| **方法名匹配** | Go 端由生成器自动转换（`GetUser` → `getUser`）；Java 端使用接口方法名 |
| **生成后不修改** | `pb/` 和 `internal/rpc/handlers_gen.go` 由工具生成，禁止手动编辑 |

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
