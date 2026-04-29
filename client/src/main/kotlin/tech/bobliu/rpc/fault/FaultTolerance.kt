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
        var lastException: Exception? = null

        for (attempt in 0..maxRetries) {
            try {
                return action()
            } catch (e: RpcException) {
                lastException = e
                if (attempt < maxRetries && isRetryable(e)) {
                    Thread.sleep(delay)
                    delay = min(delay * 2, maxDelayMs)
                } else {
                    throw e
                }
            } catch (e: Exception) {
                lastException = e
                if (attempt < maxRetries && isRetryable(e)) {
                    Thread.sleep(delay)
                    delay = min(delay * 2, maxDelayMs)
                } else {
                    throw e
                }
            }
        }

        throw RpcException(-1, "RPC failed after $maxRetries retries", lastException)
    }

    private fun isRetryable(e: Throwable): Boolean {
        return e is IOException ||
               e is SocketException ||
               e is TimeoutException ||
               (e is RpcException && isRetryable(e.cause ?: return false))
    }
}
