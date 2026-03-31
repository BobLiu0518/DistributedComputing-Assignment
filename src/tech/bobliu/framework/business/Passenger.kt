package tech.bobliu.framework.business

class Passenger(val v: Vehicle) {
    fun travel() {
        println("旅行了！")
        v.start()
    }
}