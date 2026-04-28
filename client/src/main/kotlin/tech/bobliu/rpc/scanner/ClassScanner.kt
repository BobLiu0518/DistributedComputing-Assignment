package tech.bobliu.rpc.scanner

import java.io.File
import java.net.JarURLConnection

class ClassScanner(private val basePackage: String) {

    private val classes: List<Class<*>> = scanClasses(basePackage)

    fun getClasses(): List<Class<*>> = classes

    fun <T : Annotation> getByAnnotation(annotationClass: Class<T>): List<Class<*>> =
        classes.filter { it.isAnnotationPresent(annotationClass) }

    fun <T : Annotation> associateByAnnotation(
        annotationClass: Class<T>,
        keyExtractor: ((T) -> String)? = null,
    ): Map<String, Class<*>> =
        getByAnnotation(annotationClass).associateBy { clazz ->
            clazz.getAnnotation(annotationClass)
                ?.let { keyExtractor?.invoke(it) }
                ?.takeUnless { it.isNullOrEmpty() }
                ?: clazz.simpleName
        }

    private fun scanClasses(basePackage: String): List<Class<*>> {
        val packagePath = basePackage.replace('.', '/')
        val loader = Thread.currentThread().contextClassLoader
        val classes = mutableListOf<Class<*>>()

        val resources = loader.getResources(packagePath)
        while (resources.hasMoreElements()) {
            val url = resources.nextElement()
            when (url.protocol) {
                "file" -> {
                    val root = File(url.toURI())
                    root.walkTopDown()
                        .filter { it.isFile && it.extension == "class" }
                        .forEach { file ->
                            val relative = file.relativeTo(root).invariantSeparatorsPath
                            val className =
                                "${basePackage}.${relative.removeSuffix(".class").replace('/', '.')}"
                            try {
                                classes.add(Class.forName(className, false, loader))
                            } catch (_: NoClassDefFoundError) {
                            }
                        }
                }

                "jar" -> {
                    (url.openConnection() as JarURLConnection).jarFile.use { jar ->
                        jar.entries().asSequence()
                            .filter {
                                !it.isDirectory &&
                                    it.name.startsWith("$packagePath/") &&
                                    it.name.endsWith(".class")
                            }
                            .forEach { entry ->
                                val className =
                                    entry.name.removeSuffix(".class").replace('/', '.')
                                try {
                                    classes.add(Class.forName(className, false, loader))
                                } catch (_: NoClassDefFoundError) {
                                }
                            }
                    }
                }
            }
        }
        return classes
    }
}
