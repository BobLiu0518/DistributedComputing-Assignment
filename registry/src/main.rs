mod config;
mod network;
mod proto;
mod registry;

use chrono::Local;
use config::Config;
use registry::health::HealthChecker;
use registry::notifier::Notifier;
use registry::store::ServiceStore;
use std::io::Write;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or("info"))
        .format(|buf, record| {
            writeln!(
                buf,
                "{} [{}] {}",
                Local::now().format("%Y/%m/%d %H:%M:%S"),
                record.level(),
                record.args()
            )
        })
        .init();

    let config = Config::load();
    log::info!(
        "Starting registry on {} (heartbeat_timeout={}s, health_check_interval={}s, broadcast_capacity={})",
        config.listen_addr,
        config.heartbeat_timeout_secs,
        config.health_check_interval_secs,
        config.broadcast_channel_capacity,
    );

    let store = ServiceStore::new();
    let notifier = Notifier::new(config.broadcast_channel_capacity);

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

    tokio::select! {
        result = server.run() => { result?; }
        _ = tokio::signal::ctrl_c() => {
            log::info!("Shutdown signal received, exiting");
        }
    }

    Ok(())
}
