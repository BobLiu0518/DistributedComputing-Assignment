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

- **透明调用**：`gachaService.draw(req)` —— 看起来是本地方法，实际走 TCP 远程调用
- **跨语言**：Protobuf 序列化，Java ↔ Go ↔ Rust 互通
- **Proto 唯一真相源**：协议在 proto 中定义一份，Go/Java 双端代码由 `go generate` 自动生成
- **注解驱动**：`@RpcService` / `@RpcMethod` 声明接口，无需手写网络代码
- **代码生成**：
  - Go 端 `genrpc` —— 从 proto `service` 生成强类型 Handler 接口 + 自动注册
  - Java 端 `genrpc-java` —— 从 proto `service` 生成 `@RpcService` 注解接口
- **动态代理**：JDK Proxy 拦截方法调用，自动完成序列化、网络传输、反序列化
- **嵌入式数据库**：Go 端内置 SQLite（纯 Go，零 CGO），`//go:embed` 编译进角色表
- **结构化日志**：`log/slog` 标准库，请求级别带 `request_id` / `peer` / `service` / `method`
- **服务发现**：调用端自动从注册中心获取服务地址，服务端上下线实时推送
- **负载均衡**：随机选取健康实例（`debug=true` 时输出选中实例）
- **容错**：指数退避重试（200ms → 400ms → 800ms → …）

## 快速开始

### 环境要求

- JDK 25 + Maven 3.9+
- Go 1.25+
- Rust 1.93+
- PostgreSQL 15+（`createdb rpc` 创建数据库）
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

### 3. 启动服务端（多实例）

同一份代码，不同进程，通过 `SERVICE_NAME` 区分配色：

```bash
cd server

# 用户服务（1 实例）
SERVICE_NAME=UserService RPC_PORT=8081 go run ./cmd/server/

# 抽卡服务（2 实例，演示负载均衡）
SERVICE_NAME=GachaService RPC_PORT=8082 go run ./cmd/server/   # 终端2
SERVICE_NAME=GachaService RPC_PORT=8083 go run ./cmd/server/   # 终端3
```

### 4. 运行调用端（终端交互）

```bash
cd client
mvn exec:java
```

交互示例：

```
RPC Gacha > login BobLiu 123456
  登录成功! 欢迎回来, BobLiu

RPC Gacha > draw 2
  ★★★★★★: 维什戴尔
  ★★★★★★: 缄默德克萨斯

RPC Gacha > quit
  再见~
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
  "database_url": "postgres://postgres:postgres@localhost:5432/rpc?sslmode=disable",
  "registry_addr": "localhost:9000",
  "port": 8082,
  "service_name": "GachaService",
  "heartbeat_interval_sec": 10,
  "heartbeat_max_fail": 2,
  "reconnect_backoff_sec": 1,
  "reconnect_max_backoff_sec": 30
}
```

或环境变量覆盖：

```bash
SERVICE_NAME=UserService RPC_PORT=8081 go run ./cmd/server/
```

**调用端（Java）**

修改 `@RpcApp` 注解中的 `registryHost` / `registryPort` 参数。

## 开发者指南

**Proto 是唯一真相源。** 开发者在 proto 中定义协议，其余代码由工具生成。三步完成一个新 RPC 方法：

```
1. 写 proto → 2. go generate → 3. 实现 Go handler
```

Java 端接口由 `genrpc-java` 自动生成，无需手写。

---

### 1. 定义 Protobuf

业务 proto 放在 `proto/app/`，框架 proto 在 `proto/framework/`：

```protobuf
// proto/app/user_service.proto
syntax = "proto3";
package app;
option go_package = "rpc-server/pb/app";
option java_package = "tech.bobliu.rpc.proto.app";
option java_multiple_files = true;

message RegisterRequest {
  string name = 1;
  string password = 2;
}

message RegisterResponse {
  int64 user_id = 1;
  string message = 2;
}

// ...

service UserService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Login(LoginRequest) returns (LoginResponse);
  rpc Logout(LogoutRequest) returns (LogoutResponse);
}
```

### 2. 生成代码

```bash
cd server
go generate ./cmd/server/
```

生成三类代码：

| 生成器        | 输出                                                            | 语言 |
| ------------- | --------------------------------------------------------------- | ---- |
| `genproto`    | `server/pb/` — Protobuf 消息类型                                | Go   |
| `genrpc`      | `server/internal/rpc/handlers_gen.go` — Handler 接口 + 自动注册 | Go   |
| `genrpc-java` | `client/.../app/*Rpc.java` — `@RpcService` 注解接口             | Java |

Java 端再跑一次 protobuf 编译：

```bash
cd client
mvn protobuf:compile
```

### 3. 实现 Handler（Go）

实现生成的接口——只写业务逻辑，序列化/反序列化由生成代码处理：

```go
// server/app/user_service.go
package app

import (
	"context"
	"database/sql"
	"fmt"

	pb "rpc-server/pb/app"
	"rpc-server/internal/db"
	"rpc-server/internal/rpc"
)

type UserService struct {
	db *sql.DB
}

func NewUserService(database *sql.DB) *UserService {
	return &UserService{db: database}
}

func (s *UserService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	hash := db.HashPassword(req.Password)
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO users (name, password_hash) VALUES (?, ?)", req.Name, hash)
	if err != nil {
		return &pb.RegisterResponse{Message: "注册失败: 用户名已被占用"}, nil
	}
	id, _ := result.LastInsertId()
	return &pb.RegisterResponse{
		UserId:  id,
		Message: fmt.Sprintf("注册成功! user_id=%d", id),
	}, nil
}

// 编译期检查
var _ rpc.UserServiceHandler = (*UserService)(nil)
```

GachaService 使用 `//go:embed operators.json` 将角色表编译进二进制：

```go
//go:embed operators.json
var operatorsJSON []byte

func init() {
	// 解析 JSON → rarity 分桶 → poolByRarity
}
```

### 4. 注册并启动

```go
// server/cmd/server/main.go
database, _ := db.Open(filepath.Join("data", cfg.ServiceName+".db"))

userSvc := app.NewUserService(database)
gachaSvc := app.NewGachaService(database)

r := router.New()
rpc.RegisterUserService(r, userSvc)
rpc.RegisterGachaService(r, gachaSvc)
```

---

### 调用端（Java）

接口由 `genrpc-java` 从 proto 自动生成，开发者直接使用：

```java
// 自动生成：client/.../app/UserServiceRpc.java
@RpcService("UserService")
public interface UserServiceRpc {
    @RpcMethod(requestType = RegisterRequest.class, responseType = RegisterResponse.class)
    RegisterResponse register(RegisterRequest request);
    // ...
}
```

终端交互 Demo：

```java
@RpcApp(basePackage = "tech.bobliu.rpc.app", registryHost = "localhost")
public class RpcClientDemo {
    @RpcInject
    private UserServiceRpc userService;

    @RpcInject
    private GachaServiceRpc gachaService;

    public void run() {
        // Scanner 读终端 → userService.login() / gachaService.draw()
    }
}
```

---

### 抽卡概率

先 Roll 稀有度，再在对应星级中等概率随机：

| 稀有度 | 概率 |
| ------ | ---- |
| ★★★★★★ | 2%   |
| ★★★★★  | 8%   |
| ★★★★   | 50%  |
| ★★★    | 40%  |

---

### 规范

| 规则                     | 说明                                                      |
| ------------------------ | --------------------------------------------------------- |
| **Proto 唯一真相源**     | 协议在 `proto/` 定义一份，Go/Java 双端代码由生成器产出    |
| **业务 proto 放 `app/`** | `proto/app/` 放业务协议，`proto/framework/` 放框架协议    |
| **生成后不修改**         | `pb/`、`internal/rpc/`、`*Rpc.java` 由工具生成            |
| **多实例部署**           | 同一二进制，不同 `SERVICE_NAME` + `PORT`                  |
| **编译期检查**           | Go handler 用 `var _ rpc.XxxHandler = (*XxxService)(nil)` |

## 项目结构

```
RPC/
├── proto/
│   ├── app/                      # 业务 Protobuf 协议
│   │   ├── user_service.proto    (register/login/logout)
│   │   └── gacha_service.proto   (draw)
│   └── framework/                # 框架 Protobuf 协议
│       ├── rpc.proto             (RpcRequest/Response)
│       └── registry.proto        (Register/Heartbeat)
├── registry/                     # 注册中心 (Rust)
├── server/                       # 服务端 (Go)
│   ├── app/
│   │   ├── operators.json        # 角色表（embed 编译进二进制）
│   │   ├── user_service.go       # UserService 实现
│   │   └── gacha_service.go      # GachaService 实现 + 抽卡逻辑
│   ├── cmd/
│   │   ├── server/main.go        # 入口
│   │   ├── genproto/             # protoc 封装
│   │   ├── genrpc/               # Go handler 代码生成
│   │   └── genrpc-java/          # Java 接口代码生成
│   └── internal/
│       ├── codec/                # TCP 帧编解码
│       ├── config/               # 配置加载
│       ├── db/                   # SQLite 初始化
│       ├── log/                  # slog 辅助
│       ├── registry/             # 注册中心客户端
│       ├── router/               # 方法路由
│       ├── rpc/                  # (生成) Handler 接口
│       └── server/               # TCP 服务器
├── client/                       # 调用端 (Kotlin/Java)
│   └── src/main/
│       ├── java/.../app/         # (生成) *Rpc.java 接口 + 手写 Demo
│       └── kotlin/.../           # RPC 框架 (注解/代理/注册)
└── docs/
    └── architecture.md           # 架构设计文档
```
