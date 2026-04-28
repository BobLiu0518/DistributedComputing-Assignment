package tech.bobliu.rpc.app;

import tech.bobliu.rpc.annotation.RpcApp;
import tech.bobliu.rpc.annotation.RpcInject;
import tech.bobliu.rpc.proto.example.GetUserRequest;
import tech.bobliu.rpc.proto.example.GetUserResponse;

@RpcApp(basePackage = "tech.bobliu.rpc.app")
public class RpcClientDemo {
    @RpcInject
    private UserService userService;

    public void run() {
        GetUserRequest request = GetUserRequest.newBuilder()
                .setId(42)
                .build();

        GetUserResponse response = userService.getUser(request);
        System.out.printf("User: id=%d, name=%s, age=%d%n",
                response.getId(), response.getName(), response.getAge());
    }
}
