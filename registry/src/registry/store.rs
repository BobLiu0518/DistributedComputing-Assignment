use std::collections::HashMap;
use std::sync::Arc;
use std::time::Instant;
use tokio::sync::RwLock;

use crate::proto::{ServiceInfo, ServiceList, ServiceListEntry};

#[derive(Clone, Debug)]
pub struct ServiceInstance {
    pub ip: String,
    pub port: i32,
    pub last_heartbeat: Instant,
}

pub struct ServiceStore {
    inner: Arc<RwLock<HashMap<String, Vec<ServiceInstance>>>>,
    index: Arc<RwLock<HashMap<(String, i32), String>>>,
}

impl ServiceStore {
    pub fn new() -> Self {
        Self {
            inner: Arc::new(RwLock::new(HashMap::new())),
            index: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    pub fn clone_handle(&self) -> Self {
        Self {
            inner: Arc::clone(&self.inner),
            index: Arc::clone(&self.index),
        }
    }

    pub async fn register(&self, ip: String, port: i32, service: String) -> bool {
        let mut guard = self.inner.write().await;
        let mut idx = self.index.write().await;
        let key = (ip.clone(), port);

        if let Some(old_service) = idx.get(&key) {
            if old_service == &service {
                if let Some(instances) = guard.get_mut(&service) {
                    for inst in instances.iter_mut() {
                        if inst.ip == ip && inst.port == port {
                            inst.last_heartbeat = Instant::now();
                            return false;
                        }
                    }
                }
            } else {
                if let Some(instances) = guard.get_mut(old_service) {
                    instances.retain(|i| i.ip != ip || i.port != port);
                }
            }
        }

        idx.insert(key, service.clone());
        guard.entry(service).or_default().push(ServiceInstance {
            ip,
            port,
            last_heartbeat: Instant::now(),
        });
        true
    }

    pub async fn heartbeat(&self, ip: &str, port: i32) -> bool {
        let key = (ip.to_string(), port);
        let service_name = {
            let idx = self.index.read().await;
            idx.get(&key).cloned()
        };

        let Some(service_name) = service_name else {
            return false;
        };

        let mut guard = self.inner.write().await;
        if let Some(instances) = guard.get_mut(&service_name) {
            for inst in instances.iter_mut() {
                if inst.ip == ip && inst.port == port {
                    inst.last_heartbeat = Instant::now();
                    return true;
                }
            }
        }
        false
    }

    pub async fn deregister(&self, ip: &str, port: i32) -> bool {
        let mut guard = self.inner.write().await;
        let mut idx = self.index.write().await;
        let key = (ip.to_string(), port);

        let Some(service_name) = idx.remove(&key) else {
            return false;
        };

        if let Some(instances) = guard.get_mut(&service_name) {
            instances.retain(|i| i.ip != ip || i.port != port);
            if instances.is_empty() {
                guard.remove(&service_name);
            }
        }

        true
    }

    pub async fn remove_expired(&self, timeout_secs: u64) -> Vec<(String, String, i32)> {
        let mut guard = self.inner.write().await;
        let mut idx = self.index.write().await;
        let timeout = std::time::Duration::from_secs(timeout_secs);
        let mut removed = Vec::new();

        for (service_name, instances) in guard.iter_mut() {
            let before = instances.len();
            instances.retain(|inst| {
                let alive = inst.last_heartbeat.elapsed() < timeout;
                if !alive {
                    removed.push((service_name.clone(), inst.ip.clone(), inst.port));
                    idx.remove(&(inst.ip.clone(), inst.port));
                }
                alive
            });

            if before != instances.len() {
                log::info!(
                    "Service '{}': {} instance(s) expired",
                    service_name,
                    before - instances.len()
                );
            }
        }

        guard.retain(|_, instances| !instances.is_empty());
        removed
    }

    pub async fn build_service_list(&self) -> ServiceList {
        let guard = self.inner.read().await;
        let mut services = HashMap::new();

        for (service_name, instances) in guard.iter() {
            if instances.is_empty() {
                continue;
            }
            let entry = ServiceListEntry {
                instances: instances
                    .iter()
                    .map(|inst| ServiceInfo {
                        ip: inst.ip.clone(),
                        port: inst.port,
                    })
                    .collect(),
            };
            services.insert(service_name.clone(), entry);
        }

        ServiceList { services }
    }
}
