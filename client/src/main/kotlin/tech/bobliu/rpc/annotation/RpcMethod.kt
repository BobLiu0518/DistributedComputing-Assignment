package tech.bobliu.rpc.annotation

import kotlin.reflect.KClass

@Target(AnnotationTarget.FUNCTION)
@Retention(AnnotationRetention.RUNTIME)
annotation class RpcMethod(
    val requestType: KClass<*>,
    val responseType: KClass<*>,
)
