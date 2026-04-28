package tech.bobliu.rpc.exception

class RpcException(
    val code: Int,
    override val message: String?,
    cause: Throwable? = null,
) : RuntimeException(message, cause)
