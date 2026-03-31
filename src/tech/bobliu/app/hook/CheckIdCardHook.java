package tech.bobliu.app.hook;

import tech.bobliu.framework.annotation.BeforeLodgingHook;
import tech.bobliu.framework.business.BeforeHook;

@BeforeLodgingHook("checkin")
public class CheckIdCardHook implements BeforeHook {
    @Override
    public boolean execute(String name, Object[] args) {
        int count = (int) args[0];
        if (count <= 0) {
            System.out.println("抱歉，本店暂不支持鬼魂入住");
            return false;
        }

        for (int i = 1; i <= count; i++) {
            System.out.println("第" + i + "位住客：请刷身份证");
            try {
                Thread.sleep((int) (Math.random() * 1000));
            } catch (InterruptedException _) {
            }
        }
        return true;
    }
}
