package tech.bobliu.rpc.balance

import tech.bobliu.rpc.proto.ServiceInfo
import kotlin.random.Random

object LoadBalancer {
    private val random = Random.Default

    fun select(instances: List<ServiceInfo>): ServiceInfo {
        if (instances.isEmpty()) {
            throw IllegalStateException("No available service instances")
        }
        return instances[random.nextInt(instances.size)]
    }
}
