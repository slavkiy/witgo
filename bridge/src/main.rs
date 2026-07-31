use anyhow::{Context, Result, anyhow, bail};
use serde::Deserialize;
use serde_json::{Map, Value, json};
use std::collections::BTreeMap;
use std::io::{BufRead, BufReader, BufWriter, Write, stdin, stdout};
use std::sync::mpsc::{Receiver, Sender};
use std::sync::{Arc, Mutex};
use std::time::Duration;
use wasmtime::component::types::{ComponentFunc, ComponentItem};
use wasmtime::component::{
    Component, Func, FutureAny, Instance, Linker, ResourceAny, StreamAny, Type, Val,
};
use wasmtime::{Config, Engine, Store, StoreLimits, StoreLimitsBuilder};

const PROTOCOL_VERSION: u32 = 3;
const WASMTIME_VERSION: &str = "47.0.2";
const FEATURES: &[&str] = &[
    "async-handles-v1",
    "bidirectional-handshake-v1",
    "contract-ping-v1",
    "handle-lifecycle-v1",
    "option-envelope-v1",
    "typed-signatures-v1",
];

#[derive(Deserialize)]
struct Init {
    protocol_version: u32,
    #[serde(default)]
    witgo_version: String,
    #[serde(default)]
    bridge_version: String,
    #[serde(default)]
    required_features: Vec<String>,
    component: String,
    #[serde(default)]
    imports: Vec<Import>,
    #[serde(default)]
    options: Options,
}

#[derive(Deserialize)]
struct Import {
    interface: String,
    functions: Vec<String>,
}

#[derive(Default, Deserialize)]
struct Options {
    fuel: u64,
    fuel_per_call: u64,
    timeout_millis: u64,
    memory_limit_bytes: u64,
    instance_limit: u64,
}

struct Protocol {
    io: ProtocolIo,
}

#[allow(dead_code)] // Each crate target uses one transport; lib.rs includes this file.
enum ProtocolIo {
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
    fn read(&mut self) -> Result<Value> {
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

    fn write(&mut self, value: &Value) -> Result<()> {
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

struct State {
    protocol: Arc<Mutex<Protocol>>,
    limits: StoreLimits,
}

#[derive(Clone)]
enum StoredHandle {
    Resource(ResourceAny),
    Future(FutureAny),
    Stream(StreamAny),
    ErrorContext(Val),
}

#[derive(Default)]
struct HandleTable {
    next: u64,
    values: BTreeMap<u64, StoredHandle>,
    borrowed: Vec<u64>,
}

impl HandleTable {
    fn insert(&mut self, value: StoredHandle) -> u64 {
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

    fn get(&self, id: u64) -> Result<StoredHandle> {
        self.values
            .get(&id)
            .cloned()
            .ok_or_else(|| anyhow!("component handle {id} is closed or unknown"))
    }

    fn remove(&mut self, id: u64) -> Result<StoredHandle> {
        self.values
            .remove(&id)
            .ok_or_else(|| anyhow!("component handle {id} is closed or unknown"))
    }
}

#[allow(dead_code)] // Included by the cdylib target, which exposes the C ABI instead.
fn main() {
    if let Err(error) = run() {
        let message = json!({"type": "fatal", "error": format!("{error:#}")});
        let _ = writeln!(stdout(), "{message}");
        std::process::exit(1);
    }
}

#[allow(dead_code)] // Included by the cdylib target, which uses the channel transport.
fn run() -> Result<()> {
    let protocol = Arc::new(Mutex::new(Protocol {
        io: ProtocolIo::Stdio {
            input: BufReader::new(stdin()),
            output: BufWriter::new(stdout()),
        },
    }));
    run_protocol(protocol)
}

fn run_protocol(protocol: Arc<Mutex<Protocol>>) -> Result<()> {
    let init_value = protocol.lock().unwrap().read()?;
    if init_value.get("type").and_then(Value::as_str) != Some("init") {
        bail!("first protocol message must be init")
    }
    let init: Init = serde_json::from_value(init_value).context("decode init message")?;
    if init.protocol_version != PROTOCOL_VERSION {
        protocol.lock().unwrap().write(&json!({
            "type": "error",
            "error": format!(
                "incompatible protocol: Go package {} requested version {}, bridge supports version {}",
                init.witgo_version, init.protocol_version, PROTOCOL_VERSION
            ),
            "protocol_version": PROTOCOL_VERSION,
            "witgo_version": init.witgo_version,
            "bridge_version": env!("CARGO_PKG_VERSION"),
            "wasmtime_version": WASMTIME_VERSION,
            "features": FEATURES,
        }))?;
        return Ok(());
    }
    if init.bridge_version != env!("CARGO_PKG_VERSION") {
        protocol.lock().unwrap().write(&json!({
            "type": "error",
            "error": format!(
                "incompatible bridge version: Go requires {}, bridge is {}",
                init.bridge_version, env!("CARGO_PKG_VERSION")
            ),
            "protocol_version": PROTOCOL_VERSION,
            "witgo_version": init.witgo_version,
            "bridge_version": env!("CARGO_PKG_VERSION"),
            "wasmtime_version": WASMTIME_VERSION,
            "features": FEATURES,
        }))?;
        return Ok(());
    }
    if let Some(feature) = init
        .required_features
        .iter()
        .find(|feature| !FEATURES.contains(&feature.as_str()))
    {
        protocol.lock().unwrap().write(&json!({
            "type": "error",
            "error": format!("bridge does not support required feature {feature:?}"),
            "protocol_version": PROTOCOL_VERSION,
            "witgo_version": init.witgo_version,
            "bridge_version": env!("CARGO_PKG_VERSION"),
            "wasmtime_version": WASMTIME_VERSION,
            "features": FEATURES,
        }))?;
        return Ok(());
    }

    let mut config = Config::new();
    config.wasm_component_model(true);
    config.concurrency_support(true);
    let fuel_enabled = init.options.fuel > 0 || init.options.fuel_per_call > 0;
    config.consume_fuel(fuel_enabled);
    config.epoch_interruption(init.options.timeout_millis > 0);
    let engine = Engine::new(&config).map_err(wasmtime_error)?;
    let component = Component::from_file(&engine, &init.component)
        .map_err(|error| anyhow!("load component {:?}: {error:#}", init.component))?;
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

    for import in init.imports {
        let mut instance = linker
            .instance(&import.interface)
            .map_err(|error| anyhow!("define import instance {:?}: {error:#}", import.interface))?;
        for function in import.functions {
            let interface_name = import.interface.clone();
            let function_name = function.clone();
            let handles = handles.clone();
            instance.func_new_async(&function, move |store, ty, params, results| {
                let interface_name = interface_name.clone();
                let function_name = function_name.clone();
                let handles = handles.clone();
                let protocol = store.data().protocol.clone();
                Box::new(async move {
                    handle_host_callback(
                        &protocol,
                        &interface_name,
                        &function_name,
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

    let mut limits = StoreLimitsBuilder::new();
    if init.options.memory_limit_bytes > 0 {
        limits = limits.memory_size(usize::try_from(init.options.memory_limit_bytes)?);
    }
    if init.options.instance_limit > 0 {
        limits = limits.instances(usize::try_from(init.options.instance_limit)?);
    }
    let mut store = Store::new(
        &engine,
        State {
            protocol: protocol.clone(),
            limits: limits.build(),
        },
    );
    store.limiter(|state| &mut state.limits);
    if fuel_enabled {
        let initial = if init.options.fuel_per_call > 0 {
            init.options.fuel_per_call
        } else {
            init.options.fuel
        };
        store.set_fuel(initial).map_err(wasmtime_error)?;
    }
    let instance = futures::executor::block_on(linker.instantiate_async(&mut store, &component))
        .map_err(|error| anyhow!("instantiate component: {error:#}"))?;
    futures::executor::block_on(drop_borrowed_handles(&mut store, &handles));
    protocol.lock().unwrap().write(&json!({
        "type": "ready",
    }))?;

    loop {
        let command = match protocol.lock().unwrap().read() {
            Ok(command) => command,
            Err(error) if error.to_string().contains("input closed") => return Ok(()),
            Err(error) => return Err(error),
        };
        let kind = command.get("type").and_then(Value::as_str).unwrap_or("");
        let response = match kind {
            "call" => futures::executor::block_on(handle_call(
                &engine,
                &instance,
                &mut store,
                &init.options,
                &command,
                &handles,
            )),
            "handle_drop" => match command.get("handle").and_then(Value::as_u64) {
                Some(id) => futures::executor::block_on(drop_handle(&mut store, &handles, id))
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
                futures::executor::block_on(drop_all_handles(&mut store, &handles));
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
    protocol: &Arc<Mutex<Protocol>>,
    interface: &str,
    function: &str,
    ty: &ComponentFunc,
    params: &[Val],
    results: &mut [Val],
    handles: &Arc<Mutex<HandleTable>>,
) -> Result<()> {
    let args = params
        .iter()
        .map(|value| val_to_json(value, handles))
        .collect::<Result<Vec<_>>>()?;
    let request = json!({
        "type": "host_call",
        "interface": interface,
        "function": function,
        "args": args,
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
    let result_types = ty.results().collect::<Vec<_>>();
    let values = response
        .get("values")
        .and_then(Value::as_array)
        .ok_or_else(|| anyhow!("host_result.values must be an array"))?;
    if values.len() != result_types.len() {
        bail!(
            "host returned {} results, expected {}",
            values.len(),
            result_types.len()
        )
    }
    for ((slot, value), result_ty) in results.iter_mut().zip(values).zip(result_types) {
        *slot = json_to_val(value, &result_ty, handles)?;
    }
    Ok(())
}

fn component_functions(
    engine: &Engine,
    component: &Component,
) -> (Vec<String>, Vec<String>, BTreeMap<String, String>) {
    let ty = component.component_type();
    let mut imports = Vec::new();
    let mut exports = Vec::new();
    let mut signatures = BTreeMap::new();
    for (name, item) in ty.imports(engine) {
        append_functions(engine, name, item.ty, &mut imports, &mut signatures);
    }
    for (name, item) in ty.exports(engine) {
        append_functions(engine, name, item.ty, &mut exports, &mut signatures);
    }
    imports.sort();
    exports.sort();
    (imports, exports, signatures)
}

fn append_functions(
    engine: &Engine,
    name: &str,
    item: ComponentItem,
    output: &mut Vec<String>,
    signatures: &mut BTreeMap<String, String>,
) {
    match item {
        ComponentItem::ComponentFunc(function) => {
            output.push(name.to_owned());
            signatures.insert(name.to_owned(), function_signature(&function));
        }
        ComponentItem::ComponentInstance(instance) => {
            for (function, export) in instance.exports(engine) {
                if let ComponentItem::ComponentFunc(function_type) = export.ty {
                    let full_name = format!("{name}#{function}");
                    output.push(full_name.clone());
                    signatures.insert(full_name, function_signature(&function_type));
                }
            }
        }
        _ => {}
    }
}

fn function_signature(function: &wasmtime::component::types::ComponentFunc) -> String {
    let params = function
        .params()
        .map(|(_, ty)| canonical_type(&ty))
        .collect::<Vec<_>>()
        .join(",");
    let results = function
        .results()
        .map(|ty| canonical_type(&ty))
        .collect::<Vec<_>>()
        .join(",");
    format!("({params})->({results})")
}

fn canonical_type(ty: &Type) -> String {
    match ty {
        Type::Bool => "bool".into(),
        Type::S8 => "s8".into(),
        Type::U8 => "u8".into(),
        Type::S16 => "s16".into(),
        Type::U16 => "u16".into(),
        Type::S32 => "s32".into(),
        Type::U32 => "u32".into(),
        Type::S64 => "s64".into(),
        Type::U64 => "u64".into(),
        Type::Float32 => "f32".into(),
        Type::Float64 => "f64".into(),
        Type::Char => "char".into(),
        Type::String => "string".into(),
        Type::List(list) => format!("list<{}>", canonical_type(&list.ty())),
        Type::Map(map) => format!(
            "map<{},{}>",
            canonical_type(&map.key()),
            canonical_type(&map.value())
        ),
        Type::Record(record) => format!(
            "record{{{}}}",
            record
                .fields()
                .map(|field| format!("{}:{}", field.name, canonical_type(&field.ty)))
                .collect::<Vec<_>>()
                .join(",")
        ),
        Type::Tuple(tuple) => format!(
            "tuple<{}>",
            tuple
                .types()
                .map(|ty| canonical_type(&ty))
                .collect::<Vec<_>>()
                .join(",")
        ),
        Type::Variant(variant) => format!(
            "variant{{{}}}",
            variant
                .cases()
                .map(|case| match case.ty {
                    Some(ty) => format!("{}:{}", case.name, canonical_type(&ty)),
                    None => case.name.to_owned(),
                })
                .collect::<Vec<_>>()
                .join(",")
        ),
        Type::Enum(enum_type) => format!(
            "enum{{{}}}",
            enum_type.names().collect::<Vec<_>>().join(",")
        ),
        Type::Option(option) => format!("option<{}>", canonical_type(&option.ty())),
        Type::Result(result) => format!(
            "result<{},{}>",
            result
                .ok()
                .map(|ty| canonical_type(&ty))
                .unwrap_or_default(),
            result
                .err()
                .map(|ty| canonical_type(&ty))
                .unwrap_or_default()
        ),
        Type::Flags(flags) => format!("flags{{{}}}", flags.names().collect::<Vec<_>>().join(",")),
        Type::Own(_) => "own".into(),
        Type::Borrow(_) => "borrow".into(),
        Type::Future(future) => format!(
            "future<{}>",
            future
                .ty()
                .map(|ty| canonical_type(&ty))
                .unwrap_or_default()
        ),
        Type::Stream(stream) => format!(
            "stream<{}>",
            stream
                .ty()
                .map(|ty| canonical_type(&ty))
                .unwrap_or_default()
        ),
        Type::ErrorContext => "error-context".into(),
    }
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
        store
            .set_fuel(options.fuel_per_call)
            .map_err(wasmtime_error)?;
    }
    let func = find_func(instance, store, name)?;
    let ty = func.ty(&*store);
    let param_types = ty.params().map(|(_, ty)| ty).collect::<Vec<_>>();
    if args.len() != param_types.len() {
        bail!(
            "function {name:?} received {} arguments, expected {}",
            args.len(),
            param_types.len()
        )
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
            Type::Own(_) | Type::Future(_) | Type::Stream(_) => {
                value.get("$witgo_handle").and_then(Value::as_u64)
            }
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

fn json_to_val(value: &Value, ty: &Type, handles: &Arc<Mutex<HandleTable>>) -> Result<Val> {
    Ok(match ty {
        Type::Bool => Val::Bool(value.as_bool().ok_or_else(|| anyhow!("expected bool"))?),
        Type::S8 => Val::S8(number_i64(value)?.try_into()?),
        Type::U8 => Val::U8(number_u64(value)?.try_into()?),
        Type::S16 => Val::S16(number_i64(value)?.try_into()?),
        Type::U16 => Val::U16(number_u64(value)?.try_into()?),
        Type::S32 => Val::S32(number_i64(value)?.try_into()?),
        Type::U32 => Val::U32(number_u64(value)?.try_into()?),
        Type::S64 => Val::S64(number_i64(value)?),
        Type::U64 => Val::U64(number_u64(value)?),
        Type::Float32 => {
            Val::Float32(value.as_f64().ok_or_else(|| anyhow!("expected number"))? as f32)
        }
        Type::Float64 => Val::Float64(value.as_f64().ok_or_else(|| anyhow!("expected number"))?),
        Type::Char => Val::Char(single_char(value)?),
        Type::String => Val::String(
            value
                .as_str()
                .ok_or_else(|| anyhow!("expected string"))?
                .to_owned(),
        ),
        Type::List(list) => Val::List(
            array(value)?
                .iter()
                .map(|v| json_to_val(v, &list.ty(), handles))
                .collect::<Result<_>>()?,
        ),
        Type::Map(map) => Val::Map(
            array(value)?
                .iter()
                .map(|pair| {
                    let pair = array(pair)?;
                    if pair.len() != 2 {
                        bail!("map entry must contain key and value")
                    }
                    Ok((
                        json_to_val(&pair[0], &map.key(), handles)?,
                        json_to_val(&pair[1], &map.value(), handles)?,
                    ))
                })
                .collect::<Result<_>>()?,
        ),
        Type::Record(record) => {
            let object = value
                .as_object()
                .ok_or_else(|| anyhow!("expected record object"))?;
            Val::Record(
                record
                    .fields()
                    .map(|field| {
                        let value = object
                            .get(field.name)
                            .ok_or_else(|| anyhow!("record field {:?} is missing", field.name))?;
                        Ok((
                            field.name.to_owned(),
                            json_to_val(value, &field.ty, handles)?,
                        ))
                    })
                    .collect::<Result<_>>()?,
            )
        }
        Type::Tuple(tuple) => {
            let values = array(value)?;
            let types = tuple.types().collect::<Vec<_>>();
            if values.len() != types.len() {
                bail!(
                    "tuple has {} values, expected {}",
                    values.len(),
                    types.len()
                )
            }
            Val::Tuple(
                values
                    .iter()
                    .zip(types)
                    .map(|(v, t)| json_to_val(v, &t, handles))
                    .collect::<Result<_>>()?,
            )
        }
        Type::Variant(variant) => {
            let object = value
                .as_object()
                .ok_or_else(|| anyhow!("expected variant object"))?;
            let case = object
                .get("case")
                .and_then(Value::as_str)
                .ok_or_else(|| anyhow!("variant.case is required"))?;
            let case_ty = variant
                .cases()
                .find(|item| item.name == case)
                .ok_or_else(|| anyhow!("unknown variant case {case:?}"))?
                .ty;
            let payload = match case_ty {
                Some(ty) => Some(Box::new(json_to_val(
                    object
                        .get("value")
                        .ok_or_else(|| anyhow!("variant.value is required"))?,
                    &ty,
                    handles,
                )?)),
                None => None,
            };
            Val::Variant(case.to_owned(), payload)
        }
        Type::Enum(enumeration) => {
            let name = value
                .as_str()
                .ok_or_else(|| anyhow!("expected enum string"))?;
            if !enumeration.names().any(|item| item == name) {
                bail!("unknown enum case {name:?}")
            }
            Val::Enum(name.to_owned())
        }
        Type::Option(option) => {
            let object = value
                .as_object()
                .ok_or_else(|| anyhow!("expected option object"))?;
            if object.len() != 1 {
                bail!("option must contain exactly one of some or none")
            }
            if let Some(some) = object.get("some") {
                Val::Option(Some(Box::new(json_to_val(some, &option.ty(), handles)?)))
            } else if object.get("none").and_then(Value::as_bool) == Some(true) {
                Val::Option(None)
            } else {
                bail!("option must contain some or a true none marker")
            }
        }
        Type::Result(result) => {
            let object = value
                .as_object()
                .ok_or_else(|| anyhow!("expected result object"))?;
            if let Some(ok) = object.get("ok") {
                Val::Result(Ok(match result.ok() {
                    Some(ty) => Some(Box::new(json_to_val(ok, &ty, handles)?)),
                    None => None,
                }))
            } else if let Some(err) = object.get("err") {
                Val::Result(Err(match result.err() {
                    Some(ty) => Some(Box::new(json_to_val(err, &ty, handles)?)),
                    None => None,
                }))
            } else {
                bail!("result must contain ok or err")
            }
        }
        Type::Flags(flags) => {
            let names = array(value)?
                .iter()
                .map(|v| {
                    v.as_str()
                        .map(str::to_owned)
                        .ok_or_else(|| anyhow!("flag must be a string"))
                })
                .collect::<Result<Vec<_>>>()?;
            for name in &names {
                if !flags.names().any(|item| item == name) {
                    bail!("unknown flag {name:?}")
                }
            }
            Val::Flags(names)
        }
        Type::Own(_) => take_handle(value, "resource", handles, true)?,
        Type::Borrow(_) => take_handle(value, "resource", handles, false)?,
        Type::Future(_) => take_handle(value, "future", handles, true)?,
        Type::Stream(_) => take_handle(value, "stream", handles, true)?,
        Type::ErrorContext => take_handle(value, "error-context", handles, false)?,
    })
}

fn val_to_json(value: &Val, handles: &Arc<Mutex<HandleTable>>) -> Result<Value> {
    Ok(match value {
        Val::Bool(v) => json!(v),
        Val::S8(v) => json!(v),
        Val::U8(v) => json!(v),
        Val::S16(v) => json!(v),
        Val::U16(v) => json!(v),
        Val::S32(v) => json!(v),
        Val::U32(v) => json!(v),
        Val::S64(v) => json!(v),
        Val::U64(v) => json!(v),
        Val::Float32(v) => json!(v),
        Val::Float64(v) => json!(v),
        Val::Char(v) => json!(v.to_string()),
        Val::String(v) => json!(v),
        Val::List(values) | Val::Tuple(values) => Value::Array(
            values
                .iter()
                .map(|value| val_to_json(value, handles))
                .collect::<Result<_>>()?,
        ),
        Val::Map(values) => Value::Array(
            values
                .iter()
                .map(|(k, v)| {
                    Ok(Value::Array(vec![
                        val_to_json(k, handles)?,
                        val_to_json(v, handles)?,
                    ]))
                })
                .collect::<Result<_>>()?,
        ),
        Val::Record(fields) => Value::Object(
            fields
                .iter()
                .map(|(name, value)| Ok((name.clone(), val_to_json(value, handles)?)))
                .collect::<Result<Map<_, _>>>()?,
        ),
        Val::Variant(case, value) => {
            json!({"case": case, "value": value.as_deref().map(|value| val_to_json(value, handles)).transpose()?})
        }
        Val::Enum(name) => json!(name),
        Val::Option(value) => match value.as_deref() {
            Some(value) => json!({"some": val_to_json(value, handles)?}),
            None => json!({"none": true}),
        },
        Val::Result(Ok(value)) => {
            json!({"ok": value.as_deref().map(|value| val_to_json(value, handles)).transpose()?.unwrap_or(Value::Null)})
        }
        Val::Result(Err(value)) => {
            json!({"err": value.as_deref().map(|value| val_to_json(value, handles)).transpose()?.unwrap_or(Value::Null)})
        }
        Val::Flags(values) => json!(values),
        Val::Resource(resource) => insert_handle(
            handles,
            "resource",
            resource.owned(),
            StoredHandle::Resource(*resource),
        ),
        Val::Future(future) => insert_handle(
            handles,
            "future",
            false,
            StoredHandle::Future(future.clone()),
        ),
        Val::Stream(stream) => insert_handle(
            handles,
            "stream",
            false,
            StoredHandle::Stream(stream.clone()),
        ),
        Val::ErrorContext(_) => insert_handle(
            handles,
            "error-context",
            false,
            StoredHandle::ErrorContext(value.clone()),
        ),
    })
}

fn insert_handle(
    handles: &Arc<Mutex<HandleTable>>,
    kind: &str,
    owned: bool,
    value: StoredHandle,
) -> Value {
    let id = handles.lock().unwrap().insert(value);
    json!({"$witgo_handle": id, "kind": kind, "owned": owned})
}

fn take_handle(
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

async fn drop_handle(
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

async fn drop_all_handles(store: &mut Store<State>, handles: &Arc<Mutex<HandleTable>>) {
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

async fn drop_borrowed_handles(store: &mut Store<State>, handles: &Arc<Mutex<HandleTable>>) {
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

fn array(value: &Value) -> Result<&Vec<Value>> {
    value.as_array().ok_or_else(|| anyhow!("expected array"))
}
fn number_i64(value: &Value) -> Result<i64> {
    value
        .as_i64()
        .ok_or_else(|| anyhow!("expected signed integer"))
}
fn number_u64(value: &Value) -> Result<u64> {
    value
        .as_u64()
        .ok_or_else(|| anyhow!("expected unsigned integer"))
}
fn single_char(value: &Value) -> Result<char> {
    let text = value
        .as_str()
        .ok_or_else(|| anyhow!("expected character string"))?;
    let mut chars = text.chars();
    let result = chars
        .next()
        .ok_or_else(|| anyhow!("character cannot be empty"))?;
    if chars.next().is_some() {
        bail!("expected one character")
    }
    Ok(result)
}

fn wasmtime_error(error: wasmtime::Error) -> anyhow::Error {
    anyhow!(error.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn component_ping_reports_sorted_function_names() -> Result<()> {
        let mut config = Config::new();
        config.wasm_component_model(true);
        let engine = Engine::new(&config)?;
        let component = Component::new(
            &engine,
            br#"(component
                (type $host (instance
                    (type $write (func))
                    (export "write" (func (type $write)))
                ))
                (import "test:large/host@1.0.0" (instance (type $host)))
                (core module $m (func (export "run")))
                (core instance $i (instantiate $m))
                (alias core export $i "run" (core func $run))
                (type $run-type (func))
                (func $run-component (type $run-type) (canon lift (core func $run)))
                (instance $api (export "run" (func $run-component)))
                (export "test:large/api@1.0.0" (instance $api))
            )"#,
        )?;

        let (imports, exports, signatures) = component_functions(&engine, &component);
        assert_eq!(imports, ["test:large/host@1.0.0#write"]);
        assert_eq!(exports, ["test:large/api@1.0.0#run"]);
        assert_eq!(signatures["test:large/host@1.0.0#write"], "()->()");
        Ok(())
    }
}
