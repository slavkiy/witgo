mod codec;
mod composition;
mod contract;
mod handles;
mod protocol;
mod runtime;

use protocol::{Protocol, ProtocolIo};
use runtime::run_protocol;
use serde_json::{Value, json};
use std::slice;
use std::sync::mpsc::{Receiver, Sender};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};

pub struct BridgeHandle {
    input: Sender<Value>,
    output: Receiver<Value>,
    worker: Option<JoinHandle<()>>,
}

#[unsafe(no_mangle)]
pub extern "C" fn witgo_bridge_new() -> *mut BridgeHandle {
    let (input_tx, input_rx) = std::sync::mpsc::channel();
    let (output_tx, output_rx) = std::sync::mpsc::channel();
    let fatal_tx = output_tx.clone();
    let worker = thread::spawn(move || {
        let protocol = Arc::new(Mutex::new(Protocol {
            io: ProtocolIo::Channel {
                input: input_rx,
                output: output_tx,
            },
        }));
        if let Err(error) = run_protocol(protocol) {
            let _ = fatal_tx.send(json!({"type": "fatal", "error": format!("{error:#}")}));
        }
    });
    Box::into_raw(Box::new(BridgeHandle {
        input: input_tx,
        output: output_rx,
        worker: Some(worker),
    }))
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn witgo_bridge_send(
    handle: *mut BridgeHandle,
    data: *const u8,
    len: usize,
) -> i32 {
    if handle.is_null() || data.is_null() {
        return -1;
    }
    let bytes = unsafe { slice::from_raw_parts(data, len) };
    let value: Value = match serde_json::from_slice(bytes) {
        Ok(value) => value,
        Err(_) => return -2,
    };
    match unsafe { &*handle }.input.send(value) {
        Ok(()) => 0,
        Err(_) => -3,
    }
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn witgo_bridge_receive(
    handle: *mut BridgeHandle,
    len: *mut usize,
) -> *mut u8 {
    if handle.is_null() || len.is_null() {
        return std::ptr::null_mut();
    }
    let value = match unsafe { &*handle }.output.recv() {
        Ok(value) => value,
        Err(_) => return std::ptr::null_mut(),
    };
    let bytes = match serde_json::to_vec(&value) {
        Ok(bytes) => bytes,
        Err(_) => return std::ptr::null_mut(),
    };
    unsafe {
        *len = bytes.len();
    }
    Box::into_raw(bytes.into_boxed_slice()) as *mut u8
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn witgo_bridge_free(data: *mut u8, len: usize) {
    if !data.is_null() {
        let slice = std::ptr::slice_from_raw_parts_mut(data, len);
        drop(unsafe { Box::from_raw(slice) });
    }
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn witgo_bridge_close(handle: *mut BridgeHandle) {
    if handle.is_null() {
        return;
    }
    let mut handle = unsafe { Box::from_raw(handle) };
    let _ = handle.input.send(json!({"type": "close"}));
    if let Some(worker) = handle.worker.take() {
        let _ = worker.join();
    }
}

fn wasmtime_error(error: wasmtime::Error) -> anyhow::Error {
    anyhow::anyhow!(error.to_string())
}
