package tech.bobliu.rpc.annotation

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class RpcApp(
    val basePackage: String = "",
    val registryHost: String = "localhost",
    val registryPort: Int = 9000,
    val debug: Boolean = false,
)
