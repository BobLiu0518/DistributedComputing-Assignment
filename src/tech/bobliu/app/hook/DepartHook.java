package tech.bobliu.app.hook;

import tech.bobliu.framework.annotation.BeforeTransportHook;
import tech.bobliu.framework.business.BeforeHook;

@BeforeTransportHook("start")
public class DepartHook implements BeforeHook {
    @Override
    public boolean execute(String name, Object[] args) {
        System.out.println("GO GO GO 出发喽！");
        return true;
    }
}
