package tech.bobliu.app.vehicle;

import tech.bobliu.framework.Vehicle;
import tech.bobliu.framework.annotation.Transport;

@Transport
public class Train implements Vehicle {
    @Override
    public void start() {
        System.out.println("火车开动");
        try {
            Thread.sleep(500);
        } catch (InterruptedException _) {
        }
        System.out.println("火车达速跨越" + (Math.random() < 0.5 ? "北京北" : "上海南") + "了");
    }
}
