use serde::Deserialize;

#[derive(Deserialize)]
#[serde(default)]
pub struct Config {
    pub listen_addr: String,
    pub heartbeat_timeout_secs: u64,
    pub health_check_interval_secs: u64,
    pub broadcast_channel_capacity: usize,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            listen_addr: "0.0.0.0:9000".into(),
            heartbeat_timeout_secs: 30,
            health_check_interval_secs: 10,
            broadcast_channel_capacity: 16,
        }
    }
}

impl Config {
    pub fn load() -> Self {
        let config_path = std::env::var("CARGO_MANIFEST_DIR")
            .map(|dir| std::path::PathBuf::from(dir).join("registry.toml"))
            .unwrap_or_else(|_| "registry.toml".into());

        let mut config = std::fs::read_to_string(&config_path)
            .ok()
            .and_then(|s| toml::from_str::<Config>(&s).ok())
            .unwrap_or_default();

        if let Ok(addr) = std::env::var("REGISTRY_ADDR") {
            config.listen_addr = addr;
        }
        config
    }
}
