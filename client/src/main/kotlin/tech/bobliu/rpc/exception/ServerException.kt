package tech.bobliu.rpc.exception

class ServerException(
    val code: Int,
    override val message: String?,
) : RuntimeException(message)
