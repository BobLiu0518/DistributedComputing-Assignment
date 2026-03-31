package tech.bobliu.framework

import tech.bobliu.framework.annotation.*
import tech.bobliu.framework.business.Accommodation
import tech.bobliu.framework.business.Passenger
import tech.bobliu.framework.business.Vehicle
import tech.bobliu.framework.utils.ClassScanner
import tech.bobliu.framework.utils.createProxiedInstance

fun main() {
    val classes = ClassScanner("tech.bobliu.app")
    val beforeTransportHooks = classes.getByAnnotation(BeforeTransportHook::class.java)
    val afterTransportHooks = classes.getByAnnotation(AfterTransportHook::class.java)
    val beforeLodgingHooks = classes.getByAnnotation(BeforeLodgingHook::class.java)
    val afterLodgingHooks = classes.getByAnnotation(AfterLodgingHook::class.java)

    val transports = classes.associateByAnnotation(Transport::class.java) { it.value }.mapValues {
        createProxiedInstance(it.value, Vehicle::class.java, beforeTransportHooks, afterTransportHooks)
    }
    val lodgings = classes.associateByAnnotation(Lodging::class.java) { it.value }.mapValues {
        createProxiedInstance(it.value, Accommodation::class.java, beforeLodgingHooks, afterLodgingHooks)
    }

    println("可用交通工具：${transports.keys.joinToString("、").ifEmpty { "无" }}")
    println("可用住宿类型：${lodgings.keys.joinToString("、").ifEmpty { "无" }}")
    println("输入 exit 退出")

    while (true) {
        println()
        print("请输入交通工具名称或住宿类型：")
        val input = readlnOrNull()?.trim() ?: continue

        if (input == "exit") {
            println("祝您一路顺风！")
            break
        }

        transports[input]?.let {
            Passenger(it).travel()
            continue
        }

        lodgings[input]?.let {
            print("入住人数：")
            val count = readlnOrNull()?.toIntOrNull() ?: 0
            it.checkin(count)
            continue
        }

        println("喵？");
    }
}