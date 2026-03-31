package tech.bobliu.framework.annotation

@Target(AnnotationTarget.ANNOTATION_CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class BeforeHookAnnotation

@Target(AnnotationTarget.ANNOTATION_CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class AfterHookAnnotation

@BeforeHookAnnotation
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class BeforeTransportHook(val value: Array<String> = [])

@AfterHookAnnotation
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class AfterTransportHook(val value: Array<String> = [])

@BeforeHookAnnotation
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class BeforeLodgingHook(val value: Array<String> = [])

@AfterHookAnnotation
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class AfterLodgingHook(val value: Array<String> = [])