use anyhow::{Context, Result, anyhow, bail};
use serde_json::{Value, json};
use std::sync::{Arc, Mutex};
use std::time::Duration;
use wasmtime::component::types::ComponentFunc;
use wasmtime::component::{Component, Func, Instance, Linker, Val};
use wasmtime::{Config, Engine, Store, StoreContextMut, StoreLimits, StoreLimitsBuilder};

use crate::codec::{json_to_val, val_to_json};
use crate::composition::compose_components;
use crate::contract::component_functions;
use crate::handles::{HandleTable, drop_all_handles, drop_borrowed_handles, drop_handle};
use crate::protocol::{FEATURES, Init, Options, PROTOCOL_VERSION, Protocol, WASMTIME_VERSION};
use crate::wasmtime_error;

pub struct State {
    pub protocol: Arc<Mutex<Protocol>>,
    pub limits: StoreLimits,
}

pub fn run_protocol(protocol: Arc<Mutex<Protocol>>) -> Result<()> {
    let init_value = protocol.lock().unwrap().read()?;
    if init_value.get("type").and_then(Value::as_str) != Some("init") {
        bail!("first protocol message must be init")
    }
    let init: Init = serde_json::from_value(init_value).context("decode init message")?;
    if let Some(error) = validate_init(&init) {
        protocol.lock().unwrap().write(&error)?;
        return Ok(());
    }

    let (engine, component, fuel_enabled) = load_component(&init)?;
    let (component_imports, component_exports, component_signatures) =
        component_functions(&engine, &component);
    protocol.lock().unwrap().write(&json!({
        "type": "pong",
        "protocol_version": PROTOCOL_VERSION,
        "witgo_version": init.witgo_version,
        "bridge_version": env!("CARGO_PKG_VERSION"),
        "wasmtime_version": WASMTIME_VERSION,
        "features": FEATURES,
        "imports": component_imports,
        "exports": component_exports,
        "signatures": component_signatures,
    }))?;
    let start = protocol.lock().unwrap().read()?;
    if start.get("type").and_then(Value::as_str) != Some("start") {
        bail!("expected start after contract ping")
    }

    let mut linker = Linker::<State>::new(&engine);
    let handles = Arc::new(Mutex::new(HandleTable::default()));
    register_imports(&mut linker, &init, &handles)?;
    let mut store = build_store(&engine, &init, fuel_enabled)?;
    let instance = futures::executor::block_on(linker.instantiate_async(&mut store, &component))
        .map_err(|error| anyhow!("instantiate component: {error:#}"))?;
    futures::executor::block_on(drop_borrowed_handles(&mut store, &handles));
    protocol.lock().unwrap().write(&json!({"type": "ready"}))?;

    command_loop(protocol, &engine, &instance, &mut store, &init, &handles)
}

fn validate_init(init: &Init) -> Option<Value> {
    if init.protocol_version != PROTOCOL_VERSION {
        return Some(handshake_error(
            &init.witgo_version,
            format!(
                "incompatible protocol: Go package {} requested version {}, bridge supports version {}",
                init.witgo_version, init.protocol_version, PROTOCOL_VERSION
            ),
        ));
    }
    if init.bridge_version != env!("CARGO_PKG_VERSION") {
        return Some(handshake_error(
            &init.witgo_version,
            format!(
                "incompatible bridge version: Go requires {}, bridge is {}",
                init.bridge_version, env!("CARGO_PKG_VERSION")
            ),
        ));
    }
    if let Some(feature) = init
        .required_features
        .iter()
        .find(|feature| !FEATURES.contains(&feature.as_str()))
    {
        return Some(handshake_error(
            &init.witgo_version,
            format!("bridge does not support required feature {feature:?}"),
        ));
    }
    None
}

fn handshake_error(witgo_version: &str, error: String) -> Value {
    json!({
        "type": "error",
        "error": error,
        "protocol_version": PROTOCOL_VERSION,
        "witgo_version": witgo_version,
        "bridge_version": env!("CARGO_PKG_VERSION"),
        "wasmtime_version": WASMTIME_VERSION,
        "features": FEATURES,
    })
}

fn load_component(init: &Init) -> Result<(Engine, Component, bool)> {
    let mut config = Config::new();
    config.wasm_component_model(true);
    config.wasm_component_model_map(true);
    config.concurrency_support(true);
    let fuel_enabled = init.options.fuel > 0 || init.options.fuel_per_call > 0;
    config.consume_fuel(fuel_enabled);
    config.epoch_interruption(init.options.timeout_millis > 0);
    let engine = Engine::new(&config).map_err(wasmtime_error)?;
    let component = if init.composition.is_empty() {
        Component::from_file(&engine, &init.component)
            .map_err(|error| anyhow!("load component {:?}: {error:#}", init.component))?
    } else {
        let bytes = compose_components(&init.component, &init.composition)?;
        Component::new(&engine, bytes)
            .map_err(|error| anyhow!("compile composed component {:?}: {error:#}", init.component))?
    };
    Ok((engine, component, fuel_enabled))
}

fn register_imports(
    linker: &mut Linker<State>,
    init: &Init,
    handles: &Arc<Mutex<HandleTable>>,
) -> Result<()> {
    for import in &init.imports {
        let mut instance = linker
            .instance(&import.interface)
            .map_err(|error| anyhow!("define import instance {:?}: {error:#}", import.interface))?;
        for function in &import.functions {
            let interface_name = import.interface.clone();
            let function_name = function.clone();
            let handles = handles.clone();
            instance.func_new_async(function, move |store, ty, params, results| {
                let interface_name = interface_name.clone();
                let function_name = function_name.clone();
                let handles = handles.clone();
                let protocol = store.data().protocol.clone();
                Box::new(async move {
                    handle_host_callback(
                        store,
                        &protocol,
                        (&interface_name, &function_name),
                        &ty,
                        params,
                        results,
                        &handles,
                    )
                    .map_err(wasmtime::Error::from_anyhow)
                })
            })?;
        }
    }
    Ok(())
}

fn build_store(engine: &Engine, init: &Init, fuel_enabled: bool) -> Result<Store<State>> {
    let mut limits = StoreLimitsBuilder::new();
    if init.options.memory_limit_bytes > 0 {
        limits = limits.memory_size(usize::try_from(init.options.memory_limit_bytes)?);
    }
    if init.options.instance_limit > 0 {
        limits = limits.instances(usize::try_from(init.options.instance_limit)?);
    }
    let mut store = Store::new(
        engine,
        State {
            protocol: Arc::new(Mutex::new(Protocol {
                io: crate::protocol::ProtocolIo::Channel {
                    input: std::sync::mpsc::channel().1,
                    output: std::sync::mpsc::channel().0,
                },
            })),
            limits: limits.build(),
        },
    );
    store.data_mut().protocol = Arc::new(Mutex::new(Protocol {
        io: crate::protocol::ProtocolIo::Channel {
            input: std::sync::mpsc::channel().1,
            output: std::sync::mpsc::channel().0,
        },
    }));
    store.limiter(|state| &mut state.limits);
    if fuel_enabled {
        let initial = if init.options.fuel_per_call > 0 {
            init.options.fuel_per_call
        } else {
            init.options.fuel
        };
        store.set_fuel(initial).map_err(wasmtime_error)?;
    }
    Ok(store)
}

fn command_loop(
    protocol: Arc<Mutex<Protocol>>,
    engine: &Engine,
    instance: &Instance,
    store: &mut Store<State>,
    init: &Init,
    handles: &Arc<Mutex<HandleTable>>,
) -> Result<()> {
    store.data_mut().protocol = protocol.clone();
    loop {
        let command = match protocol.lock().unwrap().read() {
            Ok(command) => command,
            Err(error) if error.to_string().contains("input closed") => return Ok(()),
            Err(error) => return Err(error),
        };
        let kind = command.get("type").and_then(Value::as_str).unwrap_or("");
        let response = match kind {
            "call" => futures::executor::block_on(handle_call(
                engine,
                instance,
                store,
                &init.options,
                &command,
                handles,
            )),
            "handle_drop" => match command.get("handle").and_then(Value::as_u64) {
                Some(id) => futures::executor::block_on(drop_handle(store, handles, id))
                    .map(|_| json!({"type": "result", "value": null})),
                None => Err(anyhow!("handle_drop.handle must be an unsigned integer")),
            },
            "fuel" => store
                .get_fuel()
                .map(|fuel| json!({"type": "result", "value": fuel}))
                .map_err(wasmtime_error),
            "set_fuel" => match command.get("fuel").and_then(Value::as_u64) {
                Some(fuel) => store
                    .set_fuel(fuel)
                    .map_err(wasmtime_error)
                    .map(|_| json!({"type": "result", "value": null})),
                None => Err(anyhow!("set_fuel.fuel must be an unsigned integer")),
            },
            "close" => {
                futures::executor::block_on(drop_all_handles(store, handles));
                return Ok(());
            }
            _ => Err(anyhow!("unknown command type {kind:?}")),
        };
        let message = match response {
            Ok(value) => value,
            Err(error) => json!({"type": "error", "error": format!("{error:#}")}),
        };
        protocol.lock().unwrap().write(&message)?;
    }
}

fn handle_host_callback(
    mut store: StoreContextMut<'_, State>,
    protocol: &Arc<Mutex<Protocol>>,
    names: (&str, &str),
    ty: &ComponentFunc,
    params: &[Val],
    results: &mut [Val],
    handles: &Arc<Mutex<HandleTable>>,
) -> Result<()> {
    let (interface, function) = names;
    let args = params
        .iter()
        .map(|value| val_to_json(value, handles))
        .collect::<Result<Vec<_>>>()?;
    let fuel_remaining = store.get_fuel().ok();
    let request = json!({
        "type": "host_call",
        "interface": interface,
        "function": function,
        "args": args,
        "fuel_enabled": fuel_remaining.is_some(),
        "fuel_remaining": fuel_remaining,
    });
    let mut io = protocol.lock().unwrap();
    io.write(&request)?;
    let response = io.read()?;
    if response.get("type").and_then(Value::as_str) != Some("host_result") {
        bail!("expected host_result response")
    }
    if let Some(error) = response.get("error").and_then(Value::as_str) {
        bail!("host function failed: {error}")
    }
    if let Some(grant) = response.get("fuel_grant").and_then(Value::as_u64) {
        let current = store.get_fuel().map_err(wasmtime_error)?;
        let next = current
            .checked_add(grant)
            .ok_or_else(|| anyhow!("fuel grant overflow"))?;
        store.set_fuel(next).map_err(wasmtime_error)?;
    }
    let result_types = ty.results().collect::<Vec<_>>();
    let values = response
        .get("values")
        .and_then(Value::as_array)
        .ok_or_else(|| anyhow!("host_result.values must be an array"))?;
    if values.len() != result_types.len() {
        bail!("host returned {} results, expected {}", values.len(), result_types.len())
    }
    for ((slot, value), result_ty) in results.iter_mut().zip(values).zip(result_types) {
        *slot = json_to_val(value, &result_ty, handles)?;
    }
    Ok(())
}

async fn handle_call(
    engine: &Engine,
    instance: &Instance,
    store: &mut Store<State>,
    options: &Options,
    command: &Value,
    handles: &Arc<Mutex<HandleTable>>,
) -> Result<Value> {
    let name = command
        .get("name")
        .and_then(Value::as_str)
        .ok_or_else(|| anyhow!("call.name is required"))?;
    let args = command
        .get("args")
        .and_then(Value::as_array)
        .ok_or_else(|| anyhow!("call.args must be an array"))?;
    if options.fuel_per_call > 0 {
        store.set_fuel(options.fuel_per_call).map_err(wasmtime_error)?;
    }
    let func = find_func(instance, store, name)?;
    let ty = func.ty(&*store);
    let param_types = ty.params().map(|(_, ty)| ty).collect::<Vec<_>>();
    if args.len() != param_types.len() {
        bail!("function {name:?} received {} arguments, expected {}", args.len(), param_types.len())
    }
    let params = args
        .iter()
        .zip(&param_types)
        .map(|(value, ty)| json_to_val(value, ty, handles))
        .collect::<Result<Vec<_>>>()?;
    let consumed = args
        .iter()
        .zip(&param_types)
        .filter_map(|(value, ty)| match ty {
            wasmtime::component::Type::Own(_)
            | wasmtime::component::Type::Future(_)
            | wasmtime::component::Type::Stream(_) => value.get("$witgo_handle").and_then(Value::as_u64),
            _ => None,
        })
        .collect::<Vec<_>>();
    let result_count = ty.results().len();
    let mut results = vec![Val::Bool(false); result_count];

    let cancel = if options.timeout_millis > 0 {
        store.set_epoch_deadline(1);
        let (tx, rx) = std::sync::mpsc::channel();
        let engine = engine.clone();
        let timeout = Duration::from_millis(options.timeout_millis);
        std::thread::spawn(move || {
            if rx.recv_timeout(timeout).is_err() {
                engine.increment_epoch();
            }
        });
        Some(tx)
    } else {
        None
    };
    let called = func.call_async(&mut *store, &params, &mut results).await;
    if let Some(cancel) = cancel {
        let _ = cancel.send(());
    }
    drop_borrowed_handles(store, handles).await;
    called.map_err(|error| anyhow!("call component function {name:?}: {error:#}"))?;
    let values = results
        .iter()
        .map(|value| val_to_json(value, handles))
        .collect::<Result<Vec<_>>>()?;
    Ok(json!({"type": "result", "values": values, "consumed": consumed}))
}

fn find_func(instance: &Instance, store: &mut Store<State>, name: &str) -> Result<Func> {
    if let Some((interface, function)) = name.rsplit_once('#') {
        let parent = instance
            .get_export_index(&mut *store, None, interface)
            .ok_or_else(|| anyhow!("component interface export {interface:?} not found"))?;
        let index = instance
            .get_export_index(&mut *store, Some(&parent), function)
            .ok_or_else(|| anyhow!("component function export {name:?} not found"))?;
        instance
            .get_func(store, index)
            .ok_or_else(|| anyhow!("component export {name:?} is not a function"))
    } else {
        instance
            .get_func(store, name)
            .ok_or_else(|| anyhow!("component function export {name:?} not found"))
    }
}
