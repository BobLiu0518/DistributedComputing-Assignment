package tech.bobliu.framework.annotation

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class Transport(val value: String = "")