package tech.bobliu.framework.utils

import tech.bobliu.framework.annotation.AfterHookAnnotation
import tech.bobliu.framework.annotation.BeforeHookAnnotation
import tech.bobliu.framework.business.AfterHook
import tech.bobliu.framework.business.BeforeHook
import java.lang.reflect.InvocationHandler
import java.lang.reflect.Method
import java.lang.reflect.Proxy

class ProxyHandler(
    private val target: Any,
    beforeHookClasses: List<Class<*>>,
    afterHookClasses: List<Class<*>>
) : InvocationHandler {
    private val beforeHooks = HashMap<String, MutableList<BeforeHook>>()
    private val afterHooks = HashMap<String, MutableList<AfterHook>>()

    init {
        collectHooks(beforeHookClasses, BeforeHook::class.java, BeforeHookAnnotation::class.java, beforeHooks)
        collectHooks(afterHookClasses, AfterHook::class.java, AfterHookAnnotation::class.java, afterHooks)
    }

    private fun <T> collectHooks(
        classes: List<Class<*>>,
        interfaceType: Class<T>,
        metaAnnotationType: Class<out Annotation>,
        targetMap: HashMap<String, MutableList<T>>,
    ) {
        classes.forEach { clazz ->
            if (!interfaceType.isAssignableFrom(clazz)) {
                throw IllegalArgumentException("类 ${clazz.name} 必须实现 ${interfaceType.simpleName} 接口")
            }

            val targetAnnotation = clazz.annotations.find { ann ->
                ann.annotationClass.java.isAnnotationPresent(metaAnnotationType)
            } ?: throw IllegalArgumentException("类 ${clazz.name} 缺少标记了 @${metaAnnotationType.simpleName} 的注解")

            val values = try {
                val valueMethod = targetAnnotation.annotationClass.java.getMethod("value")
                valueMethod.invoke(targetAnnotation) as? Array<String>
            } catch (_: NoSuchMethodException) {
                null
            }

            val instance = clazz.getDeclaredConstructor().newInstance() as T
            val keys = if (values.isNullOrEmpty()) listOf("") else values.toList()

            keys.forEach { key ->
                targetMap.getOrPut(key) { mutableListOf() }.add(instance)
            }
        }
    }

    fun <T> getHooks(map: Map<String, List<T>>, name: String): List<T> {
        return (map[""] ?: emptyList()) + (map[name] ?: emptyList())
    }

    override fun invoke(proxy: Any, method: Method, args: Array<Any?>?): Any? {
        val safeArgs = args ?: emptyArray()

        val passed = getHooks(beforeHooks, method.name).map { it.execute(method.name, safeArgs) }.all { it }
        if (!passed) return null

        val result = method.invoke(target, *safeArgs)
        getHooks(afterHooks, method.name).forEach { hook -> hook.execute(method.name, safeArgs) }
        return result
    }
}

fun <I : Any> createProxiedInstance(
    clazz: Class<*>,
    interfaceType: Class<I>,
    beforeHooks: List<Class<*>>,
    afterHooks: List<Class<*>>,
): I {
    if (!interfaceType.isAssignableFrom(clazz)) {
        throw IllegalArgumentException("类 ${clazz.name} 必须实现 ${interfaceType.simpleName} 接口")
    }

    val rawInstance = clazz.getDeclaredConstructor().newInstance() as I

    return Proxy.newProxyInstance(
        clazz.classLoader,
        arrayOf(interfaceType),
        ProxyHandler(rawInstance, beforeHooks, afterHooks)
    ) as I
}