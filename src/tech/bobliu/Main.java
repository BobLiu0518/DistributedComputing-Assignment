package tech.bobliu;

import java.util.List;

public class Main {
    static void main() {
        List<Class<?>> classes = ClassScanner.scan("tech.bobliu");
        classes.forEach(System.out::println);
    }
}
