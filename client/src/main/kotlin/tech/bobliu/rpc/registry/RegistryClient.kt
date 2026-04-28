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

class RegistryClient(
    private val registryHost: String,
    private val registryPort: Int,
) {
    private val group = NioEventLoopGroup(1)
    private var channel: Channel? = null
    private val serviceCache = ConcurrentHashMap<String, List<ServiceInfo>>()
    private val listeners = CopyOnWriteArrayList<(String, List<ServiceInfo>) -> Unit>()

    fun connect() {
        val latch = CountDownLatch(1)

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
                        RegistryHandler(latch),
                    )
                }
            })

        val future = bootstrap.connect(registryHost, registryPort).sync()
        channel = future.channel()

        val subscribeMsg = RegistryMessage.newBuilder()
            .setSubscribe(SubscribeRequest.getDefaultInstance())
            .build()
        val bytes = subscribeMsg.toByteArray()
        future.channel().writeAndFlush(Unpooled.wrappedBuffer(bytes)).sync()

        latch.await(5, TimeUnit.SECONDS)
    }

    fun getInstances(serviceName: String): List<ServiceInfo> =
        serviceCache[serviceName] ?: emptyList()

    fun onServiceUpdate(listener: (String, List<ServiceInfo>) -> Unit) {
        listeners.add(listener)
        serviceCache.forEach { (name, instances) -> listener(name, instances) }
    }

    fun shutdown() {
        channel?.close()
        group.shutdownGracefully()
    }

    private inner class RegistryHandler(
        private val initLatch: CountDownLatch,
    ) : ChannelInboundHandlerAdapter() {
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
                initLatch.countDown()
            }
        }

        override fun channelInactive(ctx: ChannelHandlerContext) {
            serviceCache.clear()
            super.channelInactive(ctx)
        }

        override fun exceptionCaught(ctx: ChannelHandlerContext, cause: Throwable) {
            ctx.close()
        }
    }
}
