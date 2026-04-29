package tech.bobliu.rpc.app;

import java.io.PrintWriter;
import java.io.StringWriter;
import java.util.Scanner;

import tech.bobliu.rpc.annotation.RpcApp;
import tech.bobliu.rpc.annotation.RpcInject;
import tech.bobliu.rpc.proto.app.Card;
import tech.bobliu.rpc.proto.app.DrawRequest;
import tech.bobliu.rpc.proto.app.DrawResponse;
import tech.bobliu.rpc.proto.app.LoginRequest;
import tech.bobliu.rpc.proto.app.LoginResponse;
import tech.bobliu.rpc.proto.app.LogoutRequest;
import tech.bobliu.rpc.proto.app.RegisterRequest;
import tech.bobliu.rpc.proto.app.RegisterResponse;

@RpcApp(basePackage = "tech.bobliu.rpc.app", registryHost = "localhost", debug = true)
public class RpcClientDemo {
    @RpcInject
    private UserServiceRpc userService;

    @RpcInject
    private GachaServiceRpc gachaService;

    private long currentUserId;
    private String currentUserName;

    public void run() {
        Scanner scanner = new Scanner(System.in);
        System.out.println("RPC Gacha System");
        System.out.println("  register <name> <password>");
        System.out.println("  login <name> <password>");
        System.out.println("  draw <1-10>");
        System.out.println("  quit");
        System.out.println();

        while (true) {
            System.out.print("RPC Gacha > ");
            String line = scanner.nextLine().trim();
            if (line.isEmpty()) continue;

            String[] parts = line.split("\\s+");
            String cmd = parts[0].toLowerCase();

            try {
                switch (cmd) {
                    case "register" -> cmdRegister(parts);
                    case "login" -> cmdLogin(parts);
                    case "draw" -> cmdDraw(parts);
                    case "quit" -> {
                        if (currentUserId > 0) {
                            userService.logout(LogoutRequest.newBuilder().setUserId(currentUserId).build());
                        }
                        System.out.println("再见~");
                        return;
                    }
                    default -> System.out.println("未知命令: " + cmd);
                }
            } catch (Exception e) {
                StringWriter sw = new StringWriter();
                e.printStackTrace(new PrintWriter(sw));
                System.out.println("错误: " + e + "\n" + sw);
            }
        }
    }

    private void cmdRegister(String[] parts) {
        if (parts.length < 3) { System.out.println("用法: register <name> <password>"); return; }
        RegisterResponse resp = userService.register(
                RegisterRequest.newBuilder().setName(parts[1]).setPassword(parts[2]).build());
        System.out.println("  " + resp.getMessage());
    }

    private void cmdLogin(String[] parts) {
        if (parts.length < 3) { System.out.println("用法: login <name> <password>"); return; }
        LoginResponse resp = userService.login(
                LoginRequest.newBuilder().setName(parts[1]).setPassword(parts[2]).build());
        System.out.println("  " + resp.getMessage());
        if (resp.getUserId() > 0) {
            currentUserId = resp.getUserId();
            currentUserName = parts[1];
        }
    }

    private void cmdDraw(String[] parts) {
        if (currentUserId == 0) { System.out.println("  请先登录!"); return; }
        int count = 1;
        if (parts.length >= 2) {
            try { count = Integer.parseInt(parts[1]); } catch (NumberFormatException e) {}
        }

        DrawResponse resp = gachaService.draw(
                DrawRequest.newBuilder().setUserId(currentUserId).setCount(count).build());
        for (Card card : resp.getCardsList()) {
            String stars = switch (card.getStars()) {
                case 6 -> "★★★★★★";
                case 5 -> "★★★★★";
                case 4 -> "★★★★";
                default -> "★★★";
            };
            System.out.printf("  %s: %s%n", stars, card.getName());
        }
    }
}
