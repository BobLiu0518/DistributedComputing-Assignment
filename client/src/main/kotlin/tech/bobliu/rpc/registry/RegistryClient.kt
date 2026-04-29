package tech.bobliu.rpc.registry

import io.netty.bootstrap.Bootstrap
import io.netty.buffer.ByteBuf
import io.netty.buffer.Unpooled
import io.netty.channel.*
import io.netty.channel.nio.NioEventLoopGroup
import io.netty.channel.socket.SocketChannel
import io.netty.channel.socket.nio.NioSocketChannel
import io.netty.handler.codec.LengthFieldBasedFrameDecoder
import io.netty.handler.codec.LengthFieldPrepender
import tech.bobliu.rpc.proto.RegistryMessage
import tech.bobliu.rpc.proto.ServiceInfo
import tech.bobliu.rpc.proto.SubscribeRequest
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlin.math.min

class RegistryClient(
    private val registryHost: String,
    private val registryPort: Int,
    private val debug: Boolean = false,
) {
    private val group = NioEventLoopGroup(1)
    @Volatile private var channel: Channel? = null
    private val serviceCache = ConcurrentHashMap<String, List<ServiceInfo>>()
    private val listeners = CopyOnWriteArrayList<(String, List<ServiceInfo>) -> Unit>()
    private var connected = CountDownLatch(1)
    @Volatile private var shutdown = false

    private fun log(msg: String) {
        if (debug) System.err.println(msg)
    }

    fun connect() {
        var delay = 1000L
        val maxDelay = 30000L
        log("[registry] connect() starting, target=$registryHost:$registryPort")

        while (!shutdown) {
            try {
                doConnect()
                log("[registry] connect() success")
                return
            } catch (e: Exception) {
                if (delay >= maxDelay || shutdown) {
                    throw RuntimeException(
                        "Failed to connect to registry after retries: ${e.message}", e
                    )
                }
                log(
                    "[registry] connect failed, retrying in ${delay}ms: ${e.message}"
                )
                Thread.sleep(delay)
                delay = min(delay * 2, maxDelay)
            }
        }
    }

    private fun doConnect() {
        connected = CountDownLatch(1)
        log("[registry] doConnect() attempting connection...")

        val bootstrap = Bootstrap()
            .group(group)
            .channel(NioSocketChannel::class.java)
            .option(ChannelOption.CONNECT_TIMEOUT_MILLIS, 5000)
            .option(ChannelOption.SO_KEEPALIVE, true)
            .handler(object : ChannelInitializer<SocketChannel>() {
                override fun initChannel(ch: SocketChannel) {
                    ch.pipeline().addLast(
                        LengthFieldBasedFrameDecoder(16 * 1024 * 1024, 0, 4, 0, 4),
                        LengthFieldPrepender(4),
                        RegistryHandler(),
                    )
                }
            })

        val future = bootstrap.connect(registryHost, registryPort).sync()
        channel = future.channel()

        val subscribeMsg = RegistryMessage.newBuilder()
            .setSubscribe(SubscribeRequest.getDefaultInstance())
            .build()
        future.channel()
            .writeAndFlush(Unpooled.wrappedBuffer(subscribeMsg.toByteArray()))
            .sync()

        if (!connected.await(5, TimeUnit.SECONDS)) {
            channel?.close()
            log("[registry] timeout waiting for ServiceList")
            throw RuntimeException(
                "Failed to receive service list from registry within 5 seconds"
            )
        }
        log("[registry] received ServiceList, connected=true")
    }

    fun getInstances(serviceName: String): List<ServiceInfo> =
        serviceCache[serviceName] ?: emptyList()

    fun onServiceUpdate(listener: (String, List<ServiceInfo>) -> Unit) {
        listeners.add(listener)
        serviceCache.forEach { (name, instances) -> listener(name, instances) }
    }

    fun shutdown() {
        shutdown = true
        channel?.close()
        group.shutdownGracefully()
    }

    private fun scheduleReconnect() {
        Thread {
            var delay = 1000L
            val maxDelay = 30000L
            var reconnected = false

            while (!shutdown && !reconnected) {
                try {
                    doConnect()
                    log("[registry] reconnected successfully")
                    reconnected = true
                } catch (e: Exception) {
                    if (shutdown) break
                    log(
                        "[registry] reconnect failed, retrying in ${delay}ms: ${e.message}"
                    )
                    try {
                        Thread.sleep(delay)
                    } catch (_: InterruptedException) {
                        break
                    }
                    delay = min(delay * 2, maxDelay)
                }
            }
        }.apply { isDaemon = true }.start()
    }

    private inner class RegistryHandler : ChannelInboundHandlerAdapter() {
        override fun channelRead(ctx: ChannelHandlerContext, msg: Any) {
            val buf = msg as ByteBuf
            val bytes = ByteArray(buf.readableBytes())
            buf.readBytes(bytes)
            buf.release()

            val registryMsg = RegistryMessage.parseFrom(bytes)
            if (registryMsg.hasServiceList()) {
                val serviceList = registryMsg.serviceList
                serviceList.servicesMap.forEach { (name, entry) ->
                    val instances = entry.instancesList
                    serviceCache[name] = instances
                    listeners.forEach { it(name, instances) }
                }
                connected.countDown()
            }
        }

        override fun channelInactive(ctx: ChannelHandlerContext) {
            val serviceNames = serviceCache.keys().toList()
            serviceCache.clear()
            for (name in serviceNames) {
                for (listener in listeners) {
                    listener(name, emptyList())
                }
            }
            scheduleReconnect()
            super.channelInactive(ctx)
        }

        override fun exceptionCaught(ctx: ChannelHandlerContext, cause: Throwable) {
            log("[registry] exception: ${cause.message}")
            ctx.close()
        }
    }
}
