package tech.bobliu.framework

import java.io.File
import java.net.JarURLConnection

fun scanClasses(basePackage: String): List<Class<*>> {
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
                        val className = "${basePackage}.${relative.removeSuffix(".class").replace('/', '.')}"
                        Class.forName(className, false, loader).let { classes.add(it) }
                    }
            }

            "jar" -> {
                (url.openConnection() as JarURLConnection).jarFile.use { jar ->
                    jar.entries().asSequence()
                        .filter { !it.isDirectory && it.name.startsWith("${packagePath}/") && it.name.endsWith(".class") }
                        .forEach { entry ->
                            val className = entry.name.removeSuffix(".class").replace('/', '.')
                            Class.forName(className, false, loader).let { classes.add(it) }
                        }
                }
            }
        }
    }
    return classes
}

fun <T : Annotation> List<Class<*>>.associateByAnnotation(
    annotationClass: Class<T>,
    keyExtractor: ((T) -> String)? = null
): Map<String, Class<*>> = this
    .filter { it.isAnnotationPresent(annotationClass) }
    .associateBy { clazz ->
        clazz.getAnnotation(annotationClass)
            ?.let { keyExtractor?.invoke(it) }
            ?.takeUnless { it.isEmpty() }
            ?: clazz.simpleName
    }