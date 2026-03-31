package tech.bobliu.app.vehicle;

import tech.bobliu.framework.annotation.Transport;
import tech.bobliu.framework.business.Vehicle;

import java.util.List;

@Transport("6路")
public class Bus implements Vehicle {
    private static final List<String> R1 = List.of("长白路图们路", "图们路控江路", "控江路敦化路", "控江路隆昌路", "控江路双阳路", "控江路黄兴路", "控江路凤城路", "新华医院", "控江路本溪路", "大连路飞虹路", "周家嘴路保定路", "周家嘴路公平路", "周家嘴路新建路", "海宁路吴淞路", "海宁路四川北路", "武进路河南北路");
    private static final List<String> R2 = List.of("武进路河南北路", "海宁路四川北路", "海宁路吴淞路", "周家嘴路新建路", "周家嘴路公平路", "周家嘴路保定路", "大连路周家嘴路", "控江路打虎山路", "控江路鞍山路", "新华医院", "控江路凤城路", "控江路黄兴路", "控江路双阳路", "控江路隆昌路", "控江路内江路", "长白路敦化路", "长白路图们路");

    private List<String> currentRoute = R1;
    private int cursor = 0;

    @Override
    public void start() {
        var nextIdx = cursor + 1;
        var isLast = nextIdx == currentRoute.size() - 1;
        var nextStation = currentRoute.get(nextIdx);

        if (cursor == 0) {
            System.out.println("欢迎乘坐6路公交车");
            System.out.println("方向 " + currentRoute.getLast());
        }

        System.out.println("下一站 " + (isLast ? "终点站 " : "") + nextStation);

        try {
            Thread.sleep((int) (Math.random() * 1000));
        } catch (InterruptedException _) {
        }

        System.out.println((isLast ? "终点站 " : "") + nextStation + " 到了");

        if (isLast) {
            currentRoute = (currentRoute == R1) ? R2 : R1;
            cursor = 0;
        } else {
            cursor++;
        }
    }
}