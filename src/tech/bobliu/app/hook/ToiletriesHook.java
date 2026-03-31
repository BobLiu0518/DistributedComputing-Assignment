package tech.bobliu.app.hook;

import tech.bobliu.framework.annotation.AfterLodgingHook;
import tech.bobliu.framework.business.AfterHook;

@AfterLodgingHook("checkin")
public class ToiletriesHook implements AfterHook {
    @Override
    public void execute(String name, Object[] args) {
        System.out.println("根据上海市要求，酒店不得主动提供一次性洗漱用品。");
        System.out.println("如有需要，请向酒店前台索取。感谢您的配合！");
    }
}
