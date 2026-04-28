use serde::Deserialize;

#[derive(Deserialize)]
#[serde(default)]
pub struct Config {
    pub listen_addr: String,
    pub heartbeat_timeout_secs: u64,
    pub health_check_interval_secs: u64,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            listen_addr: "0.0.0.0:9000".into(),
            heartbeat_timeout_secs: 30,
            health_check_interval_secs: 10,
        }
    }
}

impl Config {
    pub fn load() -> Self {
        let mut config = std::fs::read_to_string("registry.toml")
            .ok()
            .and_then(|s| toml::from_str::<Config>(&s).ok())
            .unwrap_or_default();

        if let Ok(addr) = std::env::var("REGISTRY_ADDR") {
            config.listen_addr = addr;
        }
        config
    }
}
