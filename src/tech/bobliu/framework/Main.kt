package tech.bobliu.framework

import tech.bobliu.framework.annotation.Lodging
import tech.bobliu.framework.annotation.Transport

fun main() {
    val classes = scanClasses("tech.bobliu.app")
    val vehicles = classes.associateByAnnotation(Transport::class.java) { it.value }
    val lodgings = classes.associateByAnnotation(Lodging::class.java) { it.value }

    println("可用交通工具：${vehicles.keys.joinToString("、").ifEmpty { "无" }}")
    println("可用住宿类型：${lodgings.keys.joinToString("、").ifEmpty { "无" }}")
    println("输入 exit 退出")

    while (true) {
        println()
        print("请输入交通工具名称或住宿类型：")
        val input = readlnOrNull()?.trim() ?: continue

        when {
            input == "exit" -> {
                println("祝您一路顺风！")
                break
            }

            vehicles.containsKey(input) -> {
                val trans = vehicles[input]!!.getDeclaredConstructor().newInstance() as Vehicle
                Passenger(trans).travel()
            }

            lodgings.containsKey(input) -> {
                val lodging = lodgings[input]!!.getDeclaredConstructor().newInstance() as Accommodation
                print("入住人数：")
                val count = readlnOrNull()?.toIntOrNull() ?: 0
                lodging.checkin(count)
            }

            else -> println("喵？")
        }
    }
}