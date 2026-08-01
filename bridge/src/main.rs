mod codec;
mod composition;
mod contract;
mod handles;
mod protocol;
mod runtime;

use serde_json::json;
use std::io::{BufReader, BufWriter, Write, stdin, stdout};
use std::sync::{Arc, Mutex};

use protocol::{Protocol, ProtocolIo};
use runtime::run_protocol;

fn main() {
    if let Err(error) = run() {
        let message = json!({"type": "fatal", "error": format!("{error:#}")});
        let _ = writeln!(stdout(), "{message}");
        std::process::exit(1);
    }
}

fn run() -> anyhow::Result<()> {
    let protocol = Arc::new(Mutex::new(Protocol {
        io: ProtocolIo::Stdio {
            input: BufReader::new(stdin()),
            output: BufWriter::new(stdout()),
        },
    }));
    run_protocol(protocol)
}

pub(crate) fn wasmtime_error(error: wasmtime::Error) -> anyhow::Error {
    anyhow::anyhow!(error.to_string())
}
