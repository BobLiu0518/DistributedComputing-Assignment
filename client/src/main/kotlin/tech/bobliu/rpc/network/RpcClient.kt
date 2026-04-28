package tech.bobliu.rpc.network

import com.google.protobuf.MessageLite
import io.netty.bootstrap.Bootstrap
import io.netty.buffer.ByteBuf
import io.netty.buffer.Unpooled
import io.netty.channel.*
import io.netty.channel.nio.NioEventLoopGroup
import io.netty.channel.socket.SocketChannel
import io.netty.channel.socket.nio.NioSocketChannel
import io.netty.handler.codec.LengthFieldBasedFrameDecoder
import io.netty.handler.codec.LengthFieldPrepender
import tech.bobliu.rpc.proto.RpcResponse
import java.util.concurrent.CompletableFuture
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.TimeUnit

class RpcClient {
    private val group = NioEventLoopGroup()
    private val channels = ConcurrentHashMap<String, Channel>()
    private val pendingRequests = ConcurrentHashMap<String, CompletableFuture<RpcResponse>>()

    fun call(request: MessageLite, host: String, port: Int): RpcResponse {
        val channel = getOrCreateChannel(host, port)
        val rpcRequest = request as? tech.bobliu.rpc.proto.RpcRequest
            ?: throw IllegalArgumentException("Expected RpcRequest")
        val future = CompletableFuture<RpcResponse>()
        pendingRequests[rpcRequest.requestId] = future

        try {
            val bytes = rpcRequest.toByteArray()
            channel.writeAndFlush(Unpooled.wrappedBuffer(bytes)).sync()
            return future.get(10, TimeUnit.SECONDS)
        } catch (e: Exception) {
            pendingRequests.remove(rpcRequest.requestId)
            if (e is java.util.concurrent.TimeoutException) {
                throw RuntimeException("RPC call timeout for ${rpcRequest.service}.${rpcRequest.method}")
            }
            throw RuntimeException("RPC call failed: ${e.message}", e)
        }
    }

    private fun getOrCreateChannel(host: String, port: Int): Channel {
        val key = "$host:$port"
        return channels.computeIfAbsent(key) { _ ->
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
                            ResponseHandler(pendingRequests),
                        )
                    }
                })
            try {
                bootstrap.connect(host, port).sync().channel()
            } catch (e: Exception) {
                throw RuntimeException("Failed to connect to $host:$port: ${e.message}", e)
            }
        }
    }

    fun removeChannel(host: String, port: Int) {
        val key = "$host:$port"
        channels.remove(key)?.close()
    }

    fun shutdown() {
        channels.values.forEach { it.close() }
        channels.clear()
        group.shutdownGracefully()
    }

    private class ResponseHandler(
        private val pending: ConcurrentHashMap<String, CompletableFuture<RpcResponse>>,
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
            pending.values.forEach { it.completeExceptionally(RuntimeException("Connection closed")) }
            pending.clear()
            super.channelInactive(ctx)
        }

        override fun exceptionCaught(ctx: ChannelHandlerContext, cause: Throwable) {
            ctx.close()
        }
    }
}
