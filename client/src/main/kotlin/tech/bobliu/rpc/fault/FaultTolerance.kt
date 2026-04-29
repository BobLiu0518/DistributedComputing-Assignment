package tech.bobliu.rpc.fault

import tech.bobliu.rpc.exception.RpcException
import java.io.IOException
import java.net.SocketException
import java.util.concurrent.TimeoutException
import kotlin.math.min

class FaultTolerance(
    private val maxRetries: Int = 3,
    private val baseDelayMs: Long = 200,
    private val maxDelayMs: Long = 3000,
) {
    fun <T> execute(action: () -> T): T {
        var delay = baseDelayMs

        for (attempt in 0..maxRetries) {
            try {
                return action()
            } catch (e: Throwable) {
                if (attempt == maxRetries || !isRetryable(e)) {
                    throw e
                }
                Thread.sleep(delay)
                delay = min(delay * 2, maxDelayMs)
            }
        }

        throw RpcException(-1, "RPC failed after $maxRetries retries")
    }

    private fun isRetryable(e: Throwable): Boolean {
        return when (e) {
            is IOException, is SocketException, is TimeoutException -> true
            is RpcException -> isRetryable(e.cause ?: return false)
            else -> false
        }
    }
}
