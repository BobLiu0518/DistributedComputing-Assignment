# TravelAgency（IoC + AOP 作业）

一个不依赖第三方框架的迷你“旅行社框架”，支持：

- 代订交通工具（`Vehicle`）
- 代订酒店住宿（`Accommodation`）
- 在不修改业务类源码的前提下，对核心行为做前置/后置增强（AOP）

## 功能说明

### IoC：新增类型无需改框架源码

本项目通过“**注解 + 包扫描 + 反射实例化**”完成对象注册，核心点如下：

- `ClassScanner` 扫描 `tech.bobliu.app` 包下的 class
- `@Transport` 标记交通工具实现类（实现 `Vehicle`）
- `@Lodging` 标记住宿实现类（实现 `Accommodation`）
- `Main.kt` 启动时自动收集并注册，无需手写 `if/else` 或工厂分支

因此，新增类型只需要：

1. 新建类并实现对应接口
2. 添加 `@Transport` 或 `@Lodging`

即可被系统自动发现并接入。

### AOP：增强功能无需改原始业务类

本项目通过 **JDK 动态代理** 实现 AOP，核心点如下：

- 代理入口：`createProxiedInstance(...)`
- 调用分发：`ProxyHandler`
- 增强接口：
    - 前置：`BeforeHook`（返回 `boolean`，可决定是否继续执行业务方法）
    - 后置：`AfterHook`
- 增强声明：
    - 交通：`@BeforeTransportHook` / `@AfterTransportHook`
    - 住宿：`@BeforeLodgingHook` / `@AfterLodgingHook`

Hook 类通过注解声明“作用方法名”（如 `"start"`、`"checkin"`），框架会在方法调用前后自动织入，无需修改 `Bus`、`Plane`、`Inn`、
`Hostel` 等业务类源码。

## 代码结构

```text
src/tech/bobliu/
├─ app/                     # 业务实现（可自由扩展）
│  ├─ vehicle/              # 交通工具实现
│  ├─ accommodation/        # 住宿实现
│  └─ hook/                 # AOP 增强实现
└─ framework/               # 框架核心
   ├─ annotation/           # 业务与 Hook 注解
   ├─ business/             # 领域接口（Vehicle/Accommodation/Hook）
   ├─ utils/                # ClassScanner、ProxyHandler
   └─ Main.kt               # 启动入口
```

## 快速运行

直接运行入口：`tech.bobliu.framework.MainKt`

程序启动后可在控制台输入：

- 交通工具名（例如 `Plane`、`Train`、`6路`）
- 住宿类型名（例如 `Inn`、`Hostel`）
- `exit` 退出

## 扩展示例

### 新增一种交通工具

```java

@Transport("地铁2号线")
public class Metro implements Vehicle {
    @Override
    public void start() {
        System.out.println("徐泾东站到了。");
        System.out.println("出站的乘客请手持车票，依次通过闸机验票后出站。");
        System.out.println("到刘娟美甲美睫，请从-1号口出站。");
    }
}
```

添加后无需改框架代码，启动即自动可用。

### 新增一个住宿前置检查

```java

@BeforeLodgingHook("checkin")
public class SecurityHook implements BeforeHook {
    @Override
    public boolean execute(String name, Object[] args) {
        if (Math.random() < 0.5) {
            System.out.println("住客身上有炸弹！被轰走了");
            return false;
        } else {
            System.out.println("住客安检通过，欢迎入住");
            return true;
        }
    }
}
```

添加后无需改 `Inn` / `Hostel`，`checkin` 调用会自动织入该逻辑。