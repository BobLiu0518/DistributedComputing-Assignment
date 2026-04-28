package tech.bobliu.rpc

import tech.bobliu.rpc.annotation.RpcService
import tech.bobliu.rpc.network.RpcClient
import tech.bobliu.rpc.proxy.RpcInvocationHandler
import tech.bobliu.rpc.registry.RegistryClient
import java.lang.reflect.Proxy

object RpcFramework {
    private lateinit var registryClient: RegistryClient
    private lateinit var rpcClient: RpcClient
    private var initialized = false

    @JvmStatic
    fun init(config: RpcConfig) {
        registryClient = RegistryClient(config.registryHost, config.registryPort)
        registryClient.connect()

        rpcClient = RpcClient()
        initialized = true
    }

    @JvmStatic
    fun <T : Any> createProxy(interfaceClass: Class<T>): T {
        check(initialized) { "RpcFramework not initialized. Call RpcFramework.init() first." }

        val anno = interfaceClass.getAnnotation(RpcService::class.java)
            ?: throw IllegalArgumentException("${interfaceClass.name} must be annotated with @RpcService")
        val serviceName = anno.value.ifEmpty { interfaceClass.simpleName }

        @Suppress("UNCHECKED_CAST")
        return Proxy.newProxyInstance(
            interfaceClass.classLoader,
            arrayOf<Class<*>>(interfaceClass),
            RpcInvocationHandler(serviceName, registryClient, rpcClient),
        ) as T
    }

    @JvmStatic
    fun shutdown() {
        rpcClient.shutdown()
        registryClient.shutdown()
    }
}
