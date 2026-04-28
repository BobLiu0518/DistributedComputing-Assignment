package tech.bobliu.rpc.app;

import tech.bobliu.rpc.RpcConfig;
import tech.bobliu.rpc.RpcFramework;
import tech.bobliu.rpc.proto.example.GetUserRequest;
import tech.bobliu.rpc.proto.example.GetUserResponse;

public class RpcClientDemo {
    public static void main(String[] args) {
        RpcFramework.init(new RpcConfig("localhost", 9000));

        UserService userService = RpcFramework.createProxy(UserService.class);

        GetUserRequest request = GetUserRequest.newBuilder()
                .setId(42)
                .build();

        GetUserResponse response = userService.getUser(request);
        System.out.printf("User: id=%d, name=%s, age=%d%n",
                response.getId(), response.getName(), response.getAge());

        RpcFramework.shutdown();
    }
}
