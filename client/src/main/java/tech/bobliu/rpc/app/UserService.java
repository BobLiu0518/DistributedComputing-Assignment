package tech.bobliu.rpc.app;

import tech.bobliu.rpc.annotation.RpcMethod;
import tech.bobliu.rpc.annotation.RpcService;
import tech.bobliu.rpc.proto.example.GetUserRequest;
import tech.bobliu.rpc.proto.example.GetUserResponse;

@RpcService("UserService")
public interface UserService {
    @RpcMethod(requestType = GetUserRequest.class, responseType = GetUserResponse.class)
    GetUserResponse getUser(GetUserRequest request);
}
