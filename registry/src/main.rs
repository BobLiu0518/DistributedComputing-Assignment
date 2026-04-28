mod config;
mod network;
mod proto;
mod registry;

use config::Config;
use registry::health::HealthChecker;
use registry::notifier::Notifier;
use registry::store::ServiceStore;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or("info")).init();

    let config = Config::from_env();
    log::info!(
        "Starting registry on {} (heartbeat_timeout={}s, health_check_interval={}s)",
        config.listen_addr,
        config.heartbeat_timeout_secs,
        config.health_check_interval_secs,
    );

    let store = ServiceStore::new();
    let notifier = Notifier::new();

    let health_checker = HealthChecker::new(
        store.clone_handle(),
        config.heartbeat_timeout_secs,
        config.health_check_interval_secs,
        notifier.sender(),
    );

    tokio::spawn(async move {
        health_checker.run().await;
    });

    let server = network::server::Server::new(config, store, notifier);
    server.run().await
}
