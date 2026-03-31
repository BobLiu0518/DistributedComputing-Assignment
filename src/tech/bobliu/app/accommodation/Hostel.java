package tech.bobliu.app.accommodation;

import tech.bobliu.framework.annotation.Lodging;
import tech.bobliu.framework.business.Accommodation;

@Lodging
public class Hostel implements Accommodation {
    @Override
    public void checkin(int count) {
        System.out.println(count + "个人入住一家旅社");
    }
}
