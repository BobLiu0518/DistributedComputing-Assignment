use futures::{SinkExt, StreamExt};
use tokio::net::{TcpListener, TcpStream};
use tokio::sync::broadcast;
use tokio_util::codec::Framed;

use crate::config::Config;
use crate::network::codec::RegistryCodec;
use crate::proto::{
    registry_message, RegistryMessage, RegistryResponse, ServiceList,
};
use crate::registry::notifier::Notifier;
use crate::registry::store::ServiceStore;

pub struct Server {
    config: Config,
    store: ServiceStore,
    notifier: Notifier,
}

impl Server {
    pub fn new(config: Config, store: ServiceStore, notifier: Notifier) -> Self {
        Self {
            config,
            store,
            notifier,
        }
    }

    pub async fn run(&self) -> anyhow::Result<()> {
        let listener = TcpListener::bind(&self.config.listen_addr).await?;
        log::info!("Registry listening on {}", &self.config.listen_addr);

        loop {
            let (socket, addr) = listener.accept().await?;
            log::info!("New connection from {}", addr);

            let store = self.store.clone_handle();
            let notifier_tx = self.notifier.sender();
            let mut notifier_rx = self.notifier.subscribe();

            tokio::spawn(async move {
                if let Err(e) =
                    handle_connection(socket, store, notifier_tx, &mut notifier_rx).await
                {
                    log::error!("Connection error from {}: {}", addr, e);
                }
                log::info!("Connection closed from {}", addr);
            });
        }
    }
}

async fn handle_connection(
    socket: TcpStream,
    store: ServiceStore,
    notifier_tx: broadcast::Sender<ServiceList>,
    notifier_rx: &mut broadcast::Receiver<ServiceList>,
) -> anyhow::Result<()> {
    let mut framed = Framed::new(socket, RegistryCodec::new());
    let mut is_client = false;
    let mut registered_instances: Vec<(String, i32)> = Vec::new();

    loop {
        if is_client {
            tokio::select! {
                frame = framed.next() => {
                    match frame {
                        Some(Ok(_msg)) => {}
                        Some(Err(e)) => {
                            return Err(e.into());
                        }
                        None => break,
                    }
                }
                notification = notifier_rx.recv() => {
                    match notification {
                        Ok(service_list) => {
                            let msg = RegistryMessage {
                                payload: Some(
                                    registry_message::Payload::ServiceList(service_list),
                                ),
                            };
                            if let Err(e) = framed.send(msg).await {
                                log::warn!("Failed to send notification to client: {}", e);
                                break;
                            }
                        }
                        Err(broadcast::error::RecvError::Lagged(n)) => {
                            log::warn!("Client lagged by {} messages, sending latest", n);
                            let service_list = store.build_service_list().await;
                            let msg = RegistryMessage {
                                payload: Some(
                                    registry_message::Payload::ServiceList(service_list),
                                ),
                            };
                            if let Err(e) = framed.send(msg).await {
                                log::warn!("Failed to send lagged notification: {}", e);
                                break;
                            }
                        }
                        Err(broadcast::error::RecvError::Closed) => {
                            break;
                        }
                    }
                }
            }
        } else {
            match framed.next().await {
                Some(Ok(msg)) => {
                    if let Some(payload) = msg.payload {
                        match payload {
                            registry_message::Payload::Register(req) => {
                                let changed = store
                                    .register(req.ip.clone(), req.port, req.service.clone())
                                    .await;
                                log::info!(
                                    "Registered service '{}' at {}:{} (changed={})",
                                    req.service,
                                    req.ip,
                                    req.port,
                                    changed,
                                );

                                registered_instances
                                    .push((req.ip.clone(), req.port));

                                let response = RegistryMessage {
                                    payload: Some(registry_message::Payload::Response(
                                        RegistryResponse {
                                            success: true,
                                            message: "registered".into(),
                                        },
                                    )),
                                };
                                framed.send(response).await?;

                                if changed {
                                    let service_list = store.build_service_list().await;
                                    let _ = notifier_tx.send(service_list);
                                }
                            }
                            registry_message::Payload::Heartbeat(req) => {
                                let found = store.heartbeat(&req.ip, req.port).await;
                                let response = RegistryMessage {
                                    payload: Some(registry_message::Payload::Response(
                                        RegistryResponse {
                                            success: found,
                                            message: if found {
                                                "alive".into()
                                            } else {
                                                "unknown instance".into()
                                            },
                                        },
                                    )),
                                };
                                if let Err(e) = framed.send(response).await {
                                    log::warn!("Failed to send heartbeat response: {}", e);
                                    break;
                                }
                                if !found {
                                    log::warn!(
                                        "Heartbeat from unknown instance {}:{}",
                                        req.ip,
                                        req.port,
                                    );
                                }
                            }
                            registry_message::Payload::Subscribe(_) => {
                                log::debug!("Client subscribed to service updates");
                                is_client = true;

                                let service_list = store.build_service_list().await;
                                let msg = RegistryMessage {
                                    payload: Some(
                                        registry_message::Payload::ServiceList(service_list),
                                    ),
                                };
                                framed.send(msg).await?;
                            }
                            _ => {
                                log::debug!("Ignoring unexpected message type");
                            }
                        }
                    }
                }
                Some(Err(e)) => {
                    return Err(e.into());
                }
                None => break,
            }
        }
    }

    let mut changed = false;
    for (ip, port) in &registered_instances {
        if store.deregister(ip, *port).await {
            log::info!("Deregistered {}:{} on connection close", ip, port);
            changed = true;
        }
    }
    if changed {
        let service_list = store.build_service_list().await;
        let _ = notifier_tx.send(service_list);
    }

    Ok(())
}
