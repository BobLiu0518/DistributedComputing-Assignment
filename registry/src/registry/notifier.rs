use tokio::sync::broadcast;
use crate::proto::ServiceList;

pub struct Notifier {
    tx: broadcast::Sender<ServiceList>,
}

impl Notifier {
    pub fn new() -> Self {
        let (tx, _) = broadcast::channel(16);
        Self { tx }
    }

    pub fn sender(&self) -> broadcast::Sender<ServiceList> {
        self.tx.clone()
    }

    pub fn subscribe(&self) -> broadcast::Receiver<ServiceList> {
        self.tx.subscribe()
    }

}
