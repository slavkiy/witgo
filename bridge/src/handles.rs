use anyhow::{Result, anyhow, bail};
use serde_json::{Value, json};
use std::collections::BTreeMap;
use std::sync::{Arc, Mutex};
use wasmtime::Store;
use wasmtime::component::{FutureAny, ResourceAny, StreamAny, Val};

use crate::runtime::State;
use crate::wasmtime_error;

#[derive(Clone)]
pub enum StoredHandle {
    Resource(ResourceAny),
    Future(FutureAny),
    Stream(StreamAny),
    ErrorContext(Val),
}

#[derive(Default)]
pub struct HandleTable {
    next: u64,
    pub values: BTreeMap<u64, StoredHandle>,
    pub borrowed: Vec<u64>,
}

impl HandleTable {
    pub fn insert(&mut self, value: StoredHandle) -> u64 {
        self.next = self
            .next
            .checked_add(1)
            .expect("component handle id overflow");
        if matches!(&value, StoredHandle::Resource(resource) if !resource.owned()) {
            self.borrowed.push(self.next);
        }
        self.values.insert(self.next, value);
        self.next
    }

    pub fn get(&self, id: u64) -> Result<StoredHandle> {
        self.values
            .get(&id)
            .cloned()
            .ok_or_else(|| anyhow!("component handle {id} is closed or unknown"))
    }

    pub fn remove(&mut self, id: u64) -> Result<StoredHandle> {
        self.values
            .remove(&id)
            .ok_or_else(|| anyhow!("component handle {id} is closed or unknown"))
    }
}

pub fn insert_handle(
    handles: &Arc<Mutex<HandleTable>>,
    kind: &str,
    owned: bool,
    value: StoredHandle,
) -> Value {
    let id = handles.lock().unwrap().insert(value);
    json!({"$witgo_handle": id, "kind": kind, "owned": owned})
}

pub fn take_handle(
    value: &Value,
    expected_kind: &str,
    handles: &Arc<Mutex<HandleTable>>,
    consume: bool,
) -> Result<Val> {
    let object = value
        .as_object()
        .ok_or_else(|| anyhow!("expected {expected_kind} handle object"))?;
    let id = object
        .get("$witgo_handle")
        .and_then(Value::as_u64)
        .ok_or_else(|| anyhow!("{expected_kind} handle id is required"))?;
    let kind = object
        .get("kind")
        .and_then(Value::as_str)
        .ok_or_else(|| anyhow!("component handle kind is required"))?;
    if kind != expected_kind {
        bail!("component handle kind is {kind:?}, expected {expected_kind:?}")
    }
    let stored = if consume {
        handles.lock().unwrap().remove(id)?
    } else {
        handles.lock().unwrap().get(id)?
    };
    match (expected_kind, stored) {
        ("resource", StoredHandle::Resource(value)) => Ok(Val::Resource(value)),
        ("future", StoredHandle::Future(value)) => Ok(Val::Future(value)),
        ("stream", StoredHandle::Stream(value)) => Ok(Val::Stream(value)),
        ("error-context", StoredHandle::ErrorContext(value)) => Ok(value),
        _ => bail!("component handle {id} has a different runtime kind"),
    }
}

pub async fn drop_handle(
    store: &mut Store<State>,
    handles: &Arc<Mutex<HandleTable>>,
    id: u64,
) -> Result<()> {
    let value = handles.lock().unwrap().remove(id)?;
    match value {
        StoredHandle::Resource(value) => value
            .resource_drop_async(store)
            .await
            .map_err(wasmtime_error),
        StoredHandle::Future(mut value) => value.close(store).map_err(wasmtime_error),
        StoredHandle::Stream(mut value) => value.close(store).map_err(wasmtime_error),
        StoredHandle::ErrorContext(_) => Ok(()),
    }
}

pub async fn drop_all_handles(store: &mut Store<State>, handles: &Arc<Mutex<HandleTable>>) {
    let ids = handles
        .lock()
        .unwrap()
        .values
        .keys()
        .copied()
        .collect::<Vec<_>>();
    for id in ids {
        let _ = drop_handle(store, handles, id).await;
    }
}

pub async fn drop_borrowed_handles(store: &mut Store<State>, handles: &Arc<Mutex<HandleTable>>) {
    let borrowed = {
        let mut table = handles.lock().unwrap();
        let ids = std::mem::take(&mut table.borrowed);
        ids.into_iter()
            .filter_map(|id| match table.values.remove(&id) {
                Some(StoredHandle::Resource(resource)) => Some(resource),
                _ => None,
            })
            .collect::<Vec<_>>()
    };
    for resource in borrowed {
        let _ = resource.resource_drop_async(&mut *store).await;
    }
}
