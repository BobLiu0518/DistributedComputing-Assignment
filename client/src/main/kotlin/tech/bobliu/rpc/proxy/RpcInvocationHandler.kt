package tech.bobliu.rpc.proxy

import com.google.protobuf.MessageLite
import tech.bobliu.rpc.annotation.RpcMethod
import tech.bobliu.rpc.balance.LoadBalancer
import tech.bobliu.rpc.exception.RpcException
import tech.bobliu.rpc.exception.ServerException
import tech.bobliu.rpc.fault.FaultTolerance
import tech.bobliu.rpc.network.RpcClient
import tech.bobliu.rpc.proto.RpcRequest
import tech.bobliu.rpc.registry.RegistryClient
import java.lang.reflect.InvocationHandler
import java.lang.reflect.Method
import java.util.UUID

class RpcInvocationHandler(
    private val serviceName: String,
    private val registryClient: RegistryClient,
    private val rpcClient: RpcClient,
    private val faultTolerance: FaultTolerance = FaultTolerance(),
    private val debug: Boolean = false,
) : InvocationHandler {

    override fun invoke(proxy: Any, method: Method, args: Array<out Any>?): Any? {
        if (method.declaringClass == Any::class.java) {
            return when (method.name) {
                "toString" -> "${proxy.javaClass.name}@${Integer.toHexString(System.identityHashCode(proxy))}"
                "hashCode" -> System.identityHashCode(proxy)
                "equals"   -> args?.getOrNull(0) === proxy
                else       -> throw UnsupportedOperationException("Unsupported Object method: ${method.name}")
            }
        }

        val rpcMethod = method.getAnnotation(RpcMethod::class.java)
            ?: throw RpcException(-1, "Method ${method.name} is not annotated with @RpcMethod")

        val requestObj = args?.getOrNull(0)
            ?: throw RpcException(-1, "Missing request parameter for ${method.name}")

        if (requestObj !is MessageLite) {
            throw RpcException(-1, "Request parameter must be a protobuf message")
        }

        val payload = requestObj.toByteArray()

        return faultTolerance.execute {
            val instances = registryClient.getInstances(serviceName)
            if (instances.isEmpty()) {
                throw RpcException(-1, "No available instances for service: $serviceName")
            }

            val instance = LoadBalancer.select(instances)
            if (debug) System.err.printf("[rpc] %s.%s → %s:%d%n", serviceName, method.name, instance.ip, instance.port)

            val request = RpcRequest.newBuilder()
                .setRequestId(UUID.randomUUID().toString())
                .setService(serviceName)
                .setMethod(method.name)
                .setPayload(com.google.protobuf.ByteString.copyFrom(payload))
                .build()

            val response = rpcClient.call(request, instance.ip, instance.port)

            if (response.hasError()) {
                throw ServerException(response.error.code, response.error.message)
            }

            val responseClass = rpcMethod.responseType.java
            val parseMethod = responseClass.getMethod("parseFrom", ByteArray::class.java)
            parseMethod.invoke(null, response.payload.toByteArray())
        }
    }
}
