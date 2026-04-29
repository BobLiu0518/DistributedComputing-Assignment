package tech.bobliu.rpc

import tech.bobliu.rpc.annotation.RpcApp
import tech.bobliu.rpc.annotation.RpcInject
import tech.bobliu.rpc.annotation.RpcService
import tech.bobliu.rpc.network.RpcClient
import tech.bobliu.rpc.proxy.RpcInvocationHandler
import tech.bobliu.rpc.registry.RegistryClient
import tech.bobliu.rpc.scanner.ClassScanner
import java.lang.reflect.Proxy
import java.util.concurrent.ConcurrentHashMap

class RpcFramework(
    registryHost: String,
    registryPort: Int,
    debug: Boolean,
    basePackage: String = "",
) {
    private val registryClient = RegistryClient(registryHost, registryPort, debug)
    private val rpcClient = RpcClient()
    private val proxies = ConcurrentHashMap<Class<*>, Any>()

    init {
        registryClient.connect()
        if (basePackage.isNotEmpty()) {
            val scanner = ClassScanner(basePackage)
            for (clazz in scanner.getByAnnotation(RpcService::class.java)) {
                if (clazz.isInterface) {
                    @Suppress("UNCHECKED_CAST")
                    proxies[clazz] = createProxyInternal(clazz as Class<Any>)
                }
            }
        }
    }

    fun <T : Any> service(interfaceClass: Class<T>): T {
        @Suppress("UNCHECKED_CAST")
        return proxies.getOrPut(interfaceClass) {
            createProxyInternal(interfaceClass)
        } as T
    }

    fun inject(target: Any) {
        for (field in target.javaClass.declaredFields) {
            if (field.isAnnotationPresent(RpcInject::class.java)) {
                field.isAccessible = true
                field.set(target, service(field.type))
            }
        }
    }

    private fun createProxyInternal(interfaceClass: Class<*>): Any {
        val anno = interfaceClass.getAnnotation(RpcService::class.java)
            ?: throw IllegalArgumentException("${interfaceClass.name} must be annotated with @RpcService")
        val serviceName = anno.value.ifEmpty { interfaceClass.simpleName }

        return Proxy.newProxyInstance(
            interfaceClass.classLoader,
            arrayOf<Class<*>>(interfaceClass),
            RpcInvocationHandler(serviceName, registryClient, rpcClient),
        )
    }

    fun shutdown() {
        rpcClient.shutdown()
        registryClient.shutdown()
    }

    companion object {
        @JvmStatic
        fun main(args: Array<String>) {
            val appClass = ClassScanner.findAnnotatedClass(RpcApp::class.java)
            val appAnno = appClass.getAnnotation(RpcApp::class.java)

            val basePackage = appAnno.basePackage.ifEmpty { appClass.packageName }

            val fw = RpcFramework(
                appAnno.registryHost,
                appAnno.registryPort,
                appAnno.debug,
                basePackage,
            )
            try {
                val app = appClass.getDeclaredConstructor().newInstance()
                fw.inject(app)
                appClass.getMethod("run").invoke(app)
            } finally {
                fw.shutdown()
            }
        }
    }
}
