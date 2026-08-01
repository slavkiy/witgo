use anyhow::{Context, Result, anyhow, bail};
use serde::Deserialize;
use serde_json::Value;
use std::io::{BufRead, BufReader, BufWriter, Write};
use std::sync::mpsc::{Receiver, Sender};

use crate::composition::CompositionPlug;

pub const PROTOCOL_VERSION: u32 = 3;
pub const WASMTIME_VERSION: &str = "47.0.2";
pub const FEATURES: &[&str] = &[
    "async-handles-v1",
    "bidirectional-handshake-v1",
    "call-context-v1",
    "contract-ping-v1",
    "fuel-query-v1",
    "handle-lifecycle-v1",
    "map-value-v1",
    "nested-call-safety-v1",
    "option-envelope-v1",
    "same-store-composition-v1",
    "typed-signatures-v1",
    "runtime-control-v1",
    "unsafe-fuel-request-v1",
];

#[derive(Deserialize)]
pub struct Init {
    pub protocol_version: u32,
    #[serde(default)]
    pub witgo_version: String,
    #[serde(default)]
    pub bridge_version: String,
    #[serde(default)]
    pub required_features: Vec<String>,
    pub component: String,
    #[serde(default)]
    pub composition: Vec<CompositionPlug>,
    #[serde(default)]
    pub imports: Vec<Import>,
    #[serde(default)]
    pub options: Options,
}

#[derive(Deserialize)]
pub struct Import {
    pub interface: String,
    pub functions: Vec<String>,
}

#[derive(Default, Deserialize)]
pub struct Options {
    pub fuel: u64,
    pub fuel_per_call: u64,
    pub timeout_millis: u64,
    pub memory_limit_bytes: u64,
    pub instance_limit: u64,
}

pub struct Protocol {
    pub io: ProtocolIo,
}

#[allow(dead_code)]
pub enum ProtocolIo {
    Stdio {
        input: BufReader<std::io::Stdin>,
        output: BufWriter<std::io::Stdout>,
    },
    Channel {
        input: Receiver<Value>,
        output: Sender<Value>,
    },
}

impl Protocol {
    pub fn read(&mut self) -> Result<Value> {
        match &mut self.io {
            ProtocolIo::Stdio { input, .. } => {
                let mut line = String::new();
                if input.read_line(&mut line)? == 0 {
                    bail!("protocol input closed")
                }
                serde_json::from_str(&line).context("decode protocol message")
            }
            ProtocolIo::Channel { input, .. } => {
                input.recv().map_err(|_| anyhow!("protocol input closed"))
            }
        }
    }

    pub fn write(&mut self, value: &Value) -> Result<()> {
        match &mut self.io {
            ProtocolIo::Stdio { output, .. } => {
                serde_json::to_writer(&mut *output, value)?;
                output.write_all(b"\n")?;
                output.flush()?;
                Ok(())
            }
            ProtocolIo::Channel { output, .. } => output
                .send(value.clone())
                .map_err(|_| anyhow!("protocol output closed")),
        }
    }
}
