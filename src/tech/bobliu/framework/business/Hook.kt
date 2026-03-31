package tech.bobliu.framework.business

interface BeforeHook {
    fun execute(name: String, args: Array<out Any?>): Boolean
}

interface AfterHook {
    fun execute(name: String, args: Array<out Any?>)
}