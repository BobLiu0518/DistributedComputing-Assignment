pub struct Config {
    pub listen_addr: String,
    pub heartbeat_timeout_secs: u64,
    pub health_check_interval_secs: u64,
}

impl Config {
    pub fn from_env() -> Self {
        Config {
            listen_addr: std::env::var("REGISTRY_ADDR")
                .unwrap_or_else(|_| "0.0.0.0:9000".to_string()),
            heartbeat_timeout_secs: 30,
            health_check_interval_secs: 10,
        }
    }
}
