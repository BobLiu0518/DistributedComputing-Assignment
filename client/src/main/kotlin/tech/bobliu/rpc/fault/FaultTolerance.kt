package tech.bobliu.rpc.fault

import tech.bobliu.rpc.exception.RpcException
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
                if (attempt < maxRetries) {
                    Thread.sleep(delay)
                    delay = min(delay * 2, maxDelayMs)
                }
            } catch (e: Exception) {
                lastException = e
                if (attempt < maxRetries) {
                    Thread.sleep(delay)
                    delay = min(delay * 2, maxDelayMs)
                }
            }
        }

        throw RpcException(-1, "RPC failed after $maxRetries retries", lastException)
    }
}
