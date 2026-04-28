package tech.bobliu.rpc

data class RpcConfig(
    val registryHost: String = "localhost",
    val registryPort: Int = 9000,
)
