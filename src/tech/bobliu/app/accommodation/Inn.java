package tech.bobliu.app.accommodation;

import tech.bobliu.framework.Accommodation;
import tech.bobliu.framework.annotation.Lodging;

@Lodging
public class Inn implements Accommodation {
    @Override
    public void checkin(int count) {
        System.out.println(count + "个人入住一家酒店");
    }
}
