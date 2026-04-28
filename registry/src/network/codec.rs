use anyhow::Result;
use bytes::{Buf, BufMut, BytesMut};
use prost::Message;
use tokio_util::codec::{Decoder, Encoder};

use crate::proto::RegistryMessage;

const MAX_FRAME_SIZE: usize = 16 * 1024 * 1024;
const HEADER_LEN: usize = 4;

pub struct RegistryCodec;

impl RegistryCodec {
    pub fn new() -> Self {
        Self
    }
}

impl Decoder for RegistryCodec {
    type Item = RegistryMessage;
    type Error = anyhow::Error;

    fn decode(&mut self, src: &mut BytesMut) -> Result<Option<Self::Item>, Self::Error> {
        if src.len() < HEADER_LEN {
            return Ok(None);
        }

        let len = u32::from_be_bytes([src[0], src[1], src[2], src[3]]) as usize;

        if len > MAX_FRAME_SIZE {
            return Err(anyhow::anyhow!(
                "frame size {} exceeds max {}",
                len,
                MAX_FRAME_SIZE
            ));
        }

        if src.len() < HEADER_LEN + len {
            src.reserve(HEADER_LEN + len - src.len());
            return Ok(None);
        }

        src.advance(HEADER_LEN);
        let msg = RegistryMessage::decode(&src[..len])?;
        src.advance(len);
        Ok(Some(msg))
    }
}

impl Encoder<RegistryMessage> for RegistryCodec {
    type Error = anyhow::Error;

    fn encode(&mut self, item: RegistryMessage, dst: &mut BytesMut) -> Result<(), Self::Error> {
        let bytes = item.encode_to_vec();
        dst.reserve(HEADER_LEN + bytes.len());
        dst.put_u32(bytes.len() as u32);
        dst.put_slice(&bytes);
        Ok(())
    }
}
