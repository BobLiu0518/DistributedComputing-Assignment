package tech.bobliu.rpc.network

import io.netty.bootstrap.Bootstrap
import io.netty.buffer.ByteBuf
import io.netty.buffer.Unpooled
import io.netty.channel.*
import io.netty.channel.nio.NioEventLoopGroup
import io.netty.channel.socket.SocketChannel
import io.netty.channel.socket.nio.NioSocketChannel
import io.netty.handler.codec.LengthFieldBasedFrameDecoder
import io.netty.handler.codec.LengthFieldPrepender
import io.netty.handler.timeout.IdleStateHandler
import tech.bobliu.rpc.exception.RpcException
import tech.bobliu.rpc.proto.RpcRequest
import tech.bobliu.rpc.proto.RpcResponse
import java.util.concurrent.CompletableFuture
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.TimeUnit

class RpcClient {
    private val group = NioEventLoopGroup()
    private val channels = ConcurrentHashMap<String, ChannelContext>()

    fun call(request: RpcRequest, host: String, port: Int): RpcResponse {
        val channelCtx = getOrCreateChannel(host, port)
        val future = CompletableFuture<RpcResponse>()
        channelCtx.pending[request.requestId] = future

        future.orTimeout(10, TimeUnit.SECONDS)
            .whenComplete { _, _ -> channelCtx.pending.remove(request.requestId) }

    try {
        val bytes = request.toByteArray()
        channelCtx.channel.writeAndFlush(Unpooled.wrappedBuffer(bytes))
        return future.get()
    } catch (e: Exception) {
            throw RpcException(-1, "RPC call failed: ${e.message}", e)
        }
    }

    private fun getOrCreateChannel(host: String, port: Int): ChannelContext {
        val key = "$host:$port"
        return channels.computeIfAbsent(key) { _ ->
            val pending = ConcurrentHashMap<String, CompletableFuture<RpcResponse>>()
            val bootstrap = Bootstrap()
                .group(group)
                .channel(NioSocketChannel::class.java)
                .option(ChannelOption.CONNECT_TIMEOUT_MILLIS, 5000)
                .option(ChannelOption.SO_KEEPALIVE, true)
                .handler(object : ChannelInitializer<SocketChannel>() {
                    override fun initChannel(ch: SocketChannel) {
                        ch.pipeline().addLast(
                            IdleStateHandler(0, 0, 60),
                            LengthFieldBasedFrameDecoder(16 * 1024 * 1024, 0, 4, 0, 4),
                            LengthFieldPrepender(4),
                            ResponseHandler(pending, key, channels),
                        )
                    }
                })
            try {
                val connectFuture = bootstrap.connect(host, port).await()
                if (!connectFuture.isSuccess) {
                    throw RuntimeException("Failed to connect to $host:$port", connectFuture.cause())
                }
                ChannelContext(connectFuture.channel(), pending)
            } catch (e: Exception) {
                throw RuntimeException("Failed to connect to $host:$port: ${e.message}", e)
            }
        }
    }

    fun shutdown() {
        channels.values.forEach { it.channel.close() }
        channels.clear()
        group.shutdownGracefully()
    }

    private class ChannelContext(
        val channel: Channel,
        val pending: ConcurrentHashMap<String, CompletableFuture<RpcResponse>>,
    )

    private class ResponseHandler(
        private val pending: ConcurrentHashMap<String, CompletableFuture<RpcResponse>>,
        private val channelKey: String,
        private val channels: ConcurrentHashMap<String, ChannelContext>,
    ) : ChannelInboundHandlerAdapter() {
        override fun channelRead(ctx: ChannelHandlerContext, msg: Any) {
            val buf = msg as ByteBuf
            val bytes = ByteArray(buf.readableBytes())
            buf.readBytes(bytes)
            buf.release()

            val response = RpcResponse.parseFrom(bytes)
            val future = pending.remove(response.requestId)
            future?.complete(response)
        }

        override fun channelInactive(ctx: ChannelHandlerContext) {
            channels.remove(channelKey)
            pending.values.forEach {
                it.completeExceptionally(RuntimeException("Connection closed"))
            }
            pending.clear()
            super.channelInactive(ctx)
        }

        override fun userEventTriggered(ctx: ChannelHandlerContext, evt: Any) {
            if (evt is io.netty.handler.timeout.IdleStateEvent) {
                ctx.close()
            }
        }

        override fun exceptionCaught(ctx: ChannelHandlerContext, cause: Throwable) {
            ctx.close()
        }
    }
}
