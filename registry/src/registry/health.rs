use std::time::Duration;
use tokio::sync::broadcast;

use crate::proto::ServiceList;
use crate::registry::store::ServiceStore;

pub struct HealthChecker {
    store: ServiceStore,
    timeout_secs: u64,
    interval_secs: u64,
    notifier_tx: broadcast::Sender<ServiceList>,
}

impl HealthChecker {
    pub fn new(
        store: ServiceStore,
        timeout_secs: u64,
        interval_secs: u64,
        notifier_tx: broadcast::Sender<ServiceList>,
    ) -> Self {
        Self {
            store,
            timeout_secs,
            interval_secs,
            notifier_tx,
        }
    }

    pub async fn run(&self) {
        let mut interval = tokio::time::interval(Duration::from_secs(self.interval_secs));
        loop {
            interval.tick().await;
            let removed = self.store.remove_expired(self.timeout_secs).await;
            if !removed.is_empty() {
                log::warn!(
                    "Health check: removed {} expired instance(s): {:?}",
                    removed.len(),
                    removed
                );
                let service_list = self.store.build_service_list().await;
                let _ = self.notifier_tx.send(service_list);
            }
        }
    }
}
