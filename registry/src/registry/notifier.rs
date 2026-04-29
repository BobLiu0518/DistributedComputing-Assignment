use crate::proto::ServiceList;
use tokio::sync::broadcast;

pub struct Notifier {
    tx: broadcast::Sender<ServiceList>,
}

impl Notifier {
    pub fn new(capacity: usize) -> Self {
        let (tx, _) = broadcast::channel(capacity);
        Self { tx }
    }

    pub fn sender(&self) -> broadcast::Sender<ServiceList> {
        self.tx.clone()
    }

    pub fn subscribe(&self) -> broadcast::Receiver<ServiceList> {
        self.tx.subscribe()
    }
}
