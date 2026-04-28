package tech.bobliu.rpc.scanner

import java.io.File
import java.net.JarURLConnection
import java.net.URL
import java.net.URLClassLoader

class ClassScanner(private val basePackage: String) {

    private val classes: List<Class<*>> = loadClasses(basePackage)

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

    companion object {
        fun <T : Annotation> findAnnotatedClass(annotationClass: Class<T>): Class<*> {
            val loader = Thread.currentThread().contextClassLoader
            val urls = getClasspathUrls()

            for (url in urls) {
                walkUrl(url, "", loader) { className ->
                    try {
                        val clazz = Class.forName(className, false, loader)
                        if (clazz.isAnnotationPresent(annotationClass)) {
                            return clazz
                        }
                    } catch (_: Exception) {
                    }
                }
            }
            throw RuntimeException(
                "No class annotated with @${annotationClass.simpleName} found on classpath",
            )
        }
    }

    private fun loadClasses(basePackage: String): List<Class<*>> {
        val packagePath = basePackage.replace('.', '/')
        val loader = Thread.currentThread().contextClassLoader
        val classes = mutableListOf<Class<*>>()

        val resources = loader.getResources(packagePath)
        while (resources.hasMoreElements()) {
            walkUrl(resources.nextElement(), packagePath, loader) { className ->
                try {
                    classes.add(Class.forName(className, false, loader))
                } catch (_: NoClassDefFoundError) {
                }
            }
        }
        return classes
    }
}

private fun getClasspathUrls(): Array<URL> {
    val loader = Thread.currentThread().contextClassLoader
    (loader as? URLClassLoader)?.urLs?.let { return it }

    val classpath = System.getProperty("java.class.path") ?: ""
    return classpath.split(File.pathSeparator)
        .filter { it.isNotEmpty() }
        .map { File(it).toURI().toURL() }
        .toTypedArray()
}

private inline fun walkUrl(url: URL, packagePath: String, loader: ClassLoader, action: (String) -> Unit) {
    when (url.protocol) {
        "file" -> {
            val root = File(url.toURI())
            val startDir = if (packagePath.isEmpty()) root else File(root, packagePath)
            if (startDir.exists()) {
                startDir.walkTopDown()
                    .filter { it.isFile && it.extension == "class" }
                    .forEach { file ->
                        val relative = file.relativeTo(root).invariantSeparatorsPath
                        val className = relative.removeSuffix(".class").replace('/', '.')
                        action(className)
                    }
            }
        }
        "jar" -> {
            (url.openConnection() as JarURLConnection).jarFile.use { jar ->
                jar.entries().asSequence()
                    .filter {
                        !it.isDirectory &&
                            (packagePath.isEmpty() || it.name.startsWith("$packagePath/")) &&
                            it.name.endsWith(".class")
                    }
                    .forEach { entry ->
                        val className = entry.name.removeSuffix(".class").replace('/', '.')
                        action(className)
                    }
            }
        }
    }
}
