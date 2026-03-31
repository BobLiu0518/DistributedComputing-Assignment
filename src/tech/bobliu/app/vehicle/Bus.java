package tech.bobliu.app.vehicle;

import tech.bobliu.framework.Vehicle;
import tech.bobliu.framework.annotation.Transport;

@Transport("6路")
public class Bus implements Vehicle {
    @Override
    public void start() {
        System.out.println("欢迎乘坐6路公交车");
        System.out.println("方向武进路河南北路");
        System.out.println("下一站 图们路控江路");
        try {
            Thread.sleep(500);
        } catch (InterruptedException _) {
        }
        System.out.println("图们路控江路 到了");
    }
}
