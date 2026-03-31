package tech.bobliu.app.vehicle;

import tech.bobliu.framework.annotation.Transport;
import tech.bobliu.framework.business.Vehicle;

@Transport
public class Plane implements Vehicle {
    @Override
    public void start() {
        System.out.println("飞机起飞");
        try {
            Thread.sleep(500);
        } catch (InterruptedException _) {
        }
        System.out.println(Math.random() < 0.8 ? "飞机降落" : "飞机坠机了！");
    }
}
