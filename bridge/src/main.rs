use anyhow::{Context, Result, anyhow, bail};
use serde::Deserialize;
use serde_json::{Map, Value, json};
use std::io::{BufRead, BufReader, BufWriter, Write, stdin, stdout};
use std::sync::{Arc, Mutex};
use std::time::Duration;
use wasmtime::component::{Component, Func, Instance, Linker, Type, Val};
use wasmtime::{Config, Engine, Store, StoreLimits, StoreLimitsBuilder};

#[derive(Deserialize)]
struct Init {
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
    input: BufReader<std::io::Stdin>,
    output: BufWriter<std::io::Stdout>,
}

impl Protocol {
    fn read(&mut self) -> Result<Value> {
        let mut line = String::new();
        if self.input.read_line(&mut line)? == 0 {
            bail!("protocol input closed")
        }
        serde_json::from_str(&line).context("decode protocol message")
    }

    fn write(&mut self, value: &Value) -> Result<()> {
        serde_json::to_writer(&mut self.output, value)?;
        self.output.write_all(b"\n")?;
        self.output.flush()?;
        Ok(())
    }
}

struct State {
    protocol: Arc<Mutex<Protocol>>,
    limits: StoreLimits,
}

fn main() {
    if let Err(error) = run() {
        let message = json!({"type": "fatal", "error": format!("{error:#}")});
        let _ = writeln!(stdout(), "{message}");
        std::process::exit(1);
    }
}

fn run() -> Result<()> {
    let protocol = Arc::new(Mutex::new(Protocol {
        input: BufReader::new(stdin()),
        output: BufWriter::new(stdout()),
    }));
    let init_value = protocol.lock().unwrap().read()?;
    if init_value.get("type").and_then(Value::as_str) != Some("init") {
        bail!("first protocol message must be init")
    }
    let init: Init = serde_json::from_value(init_value).context("decode init message")?;

    let mut config = Config::new();
    config.wasm_component_model(true);
    let fuel_enabled = init.options.fuel > 0 || init.options.fuel_per_call > 0;
    config.consume_fuel(fuel_enabled);
    config.epoch_interruption(init.options.timeout_millis > 0);
    let engine = Engine::new(&config).map_err(wasmtime_error)?;
    let component = Component::from_file(&engine, &init.component)
        .map_err(|error| anyhow!("load component {:?}: {error:#}", init.component))?;
    let mut linker = Linker::<State>::new(&engine);

    for import in init.imports {
        let mut instance = linker
            .instance(&import.interface)
            .map_err(|error| anyhow!("define import instance {:?}: {error:#}", import.interface))?;
        for function in import.functions {
            let interface_name = import.interface.clone();
            let function_name = function.clone();
            instance.func_new(&function, move |store, ty, params, results| {
                (|| -> Result<()> {
                    let args = params.iter().map(val_to_json).collect::<Result<Vec<_>>>()?;
                    let request = json!({
                        "type": "host_call",
                        "interface": interface_name,
                        "function": function_name,
                        "args": args,
                    });
                    let mut io = store.data().protocol.lock().unwrap();
                    io.write(&request)?;
                    let response = io.read()?;
                    if response.get("type").and_then(Value::as_str) != Some("host_result") {
                        bail!("expected host_result response")
                    }
                    if let Some(error) = response.get("error").and_then(Value::as_str) {
                        bail!("host function failed: {error}")
                    }
                    let result_types = ty.results().collect::<Vec<_>>();
                    let values = response.get("values").and_then(Value::as_array)
                        .ok_or_else(|| anyhow!("host_result.values must be an array"))?;
                    if values.len() != result_types.len() {
                        bail!("host returned {} results, expected {}", values.len(), result_types.len())
                    }
                    for ((slot, value), result_ty) in results.iter_mut().zip(values).zip(result_types) {
                        *slot = json_to_val(value, &result_ty)?;
                    }
                    Ok(())
                })().map_err(wasmtime::Error::from_anyhow)
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
    let mut store = Store::new(&engine, State { protocol: protocol.clone(), limits: limits.build() });
    store.limiter(|state| &mut state.limits);
    if fuel_enabled {
        let initial = if init.options.fuel_per_call > 0 { init.options.fuel_per_call } else { init.options.fuel };
        store.set_fuel(initial).map_err(wasmtime_error)?;
    }
    let instance = linker.instantiate(&mut store, &component)
        .map_err(|error| anyhow!("instantiate component: {error:#}"))?;
    protocol.lock().unwrap().write(&json!({"type": "ready"}))?;

    loop {
        let command = match protocol.lock().unwrap().read() {
            Ok(command) => command,
            Err(error) if error.to_string().contains("input closed") => return Ok(()),
            Err(error) => return Err(error),
        };
        let kind = command.get("type").and_then(Value::as_str).unwrap_or("");
        let response = match kind {
            "call" => handle_call(&engine, &instance, &mut store, &init.options, &command),
            "fuel" => store.get_fuel().map(|fuel| json!({"type": "result", "value": fuel})).map_err(wasmtime_error),
            "set_fuel" => match command.get("fuel").and_then(Value::as_u64) {
                Some(fuel) => store.set_fuel(fuel).map_err(wasmtime_error).map(|_| json!({"type": "result", "value": null})),
                None => Err(anyhow!("set_fuel.fuel must be an unsigned integer")),
            },
            "close" => return Ok(()),
            _ => Err(anyhow!("unknown command type {kind:?}")),
        };
        let message = match response {
            Ok(value) => value,
            Err(error) => json!({"type": "error", "error": format!("{error:#}")}),
        };
        protocol.lock().unwrap().write(&message)?;
    }
}

fn handle_call(engine: &Engine, instance: &Instance, store: &mut Store<State>, options: &Options, command: &Value) -> Result<Value> {
    let name = command.get("name").and_then(Value::as_str).ok_or_else(|| anyhow!("call.name is required"))?;
    let args = command.get("args").and_then(Value::as_array).ok_or_else(|| anyhow!("call.args must be an array"))?;
    if options.fuel_per_call > 0 {
        store.set_fuel(options.fuel_per_call).map_err(wasmtime_error)?;
    }
    let func = find_func(instance, store, name)?;
    let ty = func.ty(&*store);
    let param_types = ty.params().map(|(_, ty)| ty).collect::<Vec<_>>();
    if args.len() != param_types.len() {
        bail!("function {name:?} received {} arguments, expected {}", args.len(), param_types.len())
    }
    let params = args.iter().zip(&param_types)
        .map(|(value, ty)| json_to_val(value, ty)).collect::<Result<Vec<_>>>()?;
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
    let called = func.call(store, &params, &mut results);
    if let Some(cancel) = cancel { let _ = cancel.send(()); }
    called.map_err(|error| anyhow!("call component function {name:?}: {error:#}"))?;
    let values = results.iter().map(val_to_json).collect::<Result<Vec<_>>>()?;
    Ok(json!({"type": "result", "values": values}))
}

fn find_func(instance: &Instance, store: &mut Store<State>, name: &str) -> Result<Func> {
    if let Some((interface, function)) = name.rsplit_once('#') {
        let parent = instance.get_export_index(&mut *store, None, interface)
            .ok_or_else(|| anyhow!("component interface export {interface:?} not found"))?;
        let index = instance.get_export_index(&mut *store, Some(&parent), function)
            .ok_or_else(|| anyhow!("component function export {name:?} not found"))?;
        instance.get_func(store, &index).ok_or_else(|| anyhow!("component export {name:?} is not a function"))
    } else {
        instance.get_func(store, name).ok_or_else(|| anyhow!("component function export {name:?} not found"))
    }
}

fn json_to_val(value: &Value, ty: &Type) -> Result<Val> {
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
        Type::Float32 => Val::Float32(value.as_f64().ok_or_else(|| anyhow!("expected number"))? as f32),
        Type::Float64 => Val::Float64(value.as_f64().ok_or_else(|| anyhow!("expected number"))?),
        Type::Char => Val::Char(single_char(value)?),
        Type::String => Val::String(value.as_str().ok_or_else(|| anyhow!("expected string"))?.to_owned()),
        Type::List(list) => Val::List(array(value)?.iter().map(|v| json_to_val(v, &list.ty())).collect::<Result<_>>()?),
        Type::Map(map) => Val::Map(array(value)?.iter().map(|pair| {
            let pair = array(pair)?;
            if pair.len() != 2 { bail!("map entry must contain key and value") }
            Ok((json_to_val(&pair[0], &map.key())?, json_to_val(&pair[1], &map.value())?))
        }).collect::<Result<_>>()?),
        Type::Record(record) => {
            let object = value.as_object().ok_or_else(|| anyhow!("expected record object"))?;
            Val::Record(record.fields().map(|field| {
                let value = object.get(field.name).ok_or_else(|| anyhow!("record field {:?} is missing", field.name))?;
                Ok((field.name.to_owned(), json_to_val(value, &field.ty)?))
            }).collect::<Result<_>>()?)
        }
        Type::Tuple(tuple) => {
            let values = array(value)?;
            let types = tuple.types().collect::<Vec<_>>();
            if values.len() != types.len() { bail!("tuple has {} values, expected {}", values.len(), types.len()) }
            Val::Tuple(values.iter().zip(types).map(|(v, t)| json_to_val(v, &t)).collect::<Result<_>>()?)
        }
        Type::Variant(variant) => {
            let object = value.as_object().ok_or_else(|| anyhow!("expected variant object"))?;
            let case = object.get("case").and_then(Value::as_str).ok_or_else(|| anyhow!("variant.case is required"))?;
            let case_ty = variant.cases().find(|item| item.name == case).ok_or_else(|| anyhow!("unknown variant case {case:?}"))?.ty;
            let payload = match case_ty { Some(ty) => Some(Box::new(json_to_val(object.get("value").ok_or_else(|| anyhow!("variant.value is required"))?, &ty)?)), None => None };
            Val::Variant(case.to_owned(), payload)
        }
        Type::Enum(enumeration) => {
            let name = value.as_str().ok_or_else(|| anyhow!("expected enum string"))?;
            if !enumeration.names().any(|item| item == name) { bail!("unknown enum case {name:?}") }
            Val::Enum(name.to_owned())
        }
        Type::Option(option) => Val::Option(if value.is_null() { None } else { Some(Box::new(json_to_val(value, &option.ty())?)) }),
        Type::Result(result) => {
            let object = value.as_object().ok_or_else(|| anyhow!("expected result object"))?;
            if let Some(ok) = object.get("ok") {
                Val::Result(Ok(match result.ok() { Some(ty) => Some(Box::new(json_to_val(ok, &ty)?)), None => None }))
            } else if let Some(err) = object.get("err") {
                Val::Result(Err(match result.err() { Some(ty) => Some(Box::new(json_to_val(err, &ty)?)), None => None }))
            } else { bail!("result must contain ok or err") }
        }
        Type::Flags(flags) => {
            let names = array(value)?.iter().map(|v| v.as_str().map(str::to_owned).ok_or_else(|| anyhow!("flag must be a string"))).collect::<Result<Vec<_>>>()?;
            for name in &names { if !flags.names().any(|item| item == name) { bail!("unknown flag {name:?}") } }
            Val::Flags(names)
        }
        Type::Own(_) | Type::Borrow(_) | Type::Future(_) | Type::Stream(_) | Type::ErrorContext => bail!("this Component Model handle type is not supported by the JSON bridge"),
    })
}

fn val_to_json(value: &Val) -> Result<Value> {
    Ok(match value {
        Val::Bool(v) => json!(v), Val::S8(v) => json!(v), Val::U8(v) => json!(v),
        Val::S16(v) => json!(v), Val::U16(v) => json!(v), Val::S32(v) => json!(v),
        Val::U32(v) => json!(v), Val::S64(v) => json!(v), Val::U64(v) => json!(v),
        Val::Float32(v) => json!(v), Val::Float64(v) => json!(v), Val::Char(v) => json!(v.to_string()),
        Val::String(v) => json!(v),
        Val::List(values) | Val::Tuple(values) => Value::Array(values.iter().map(val_to_json).collect::<Result<_>>()?),
        Val::Map(values) => Value::Array(values.iter().map(|(k, v)| Ok(Value::Array(vec![val_to_json(k)?, val_to_json(v)?]))).collect::<Result<_>>()?),
        Val::Record(fields) => Value::Object(fields.iter().map(|(name, value)| Ok((name.clone(), val_to_json(value)?))).collect::<Result<Map<_, _>>>()?),
        Val::Variant(case, value) => json!({"case": case, "value": value.as_deref().map(val_to_json).transpose()?}),
        Val::Enum(name) => json!(name),
        Val::Option(value) => value.as_deref().map(val_to_json).transpose()?.unwrap_or(Value::Null),
        Val::Result(Ok(value)) => json!({"ok": value.as_deref().map(val_to_json).transpose()?.unwrap_or(Value::Null)}),
        Val::Result(Err(value)) => json!({"err": value.as_deref().map(val_to_json).transpose()?.unwrap_or(Value::Null)}),
        Val::Flags(values) => json!(values),
        Val::Resource(_) | Val::Future(_) | Val::Stream(_) | Val::ErrorContext(_) => bail!("this Component Model handle value is not supported by the JSON bridge"),
    })
}

fn array(value: &Value) -> Result<&Vec<Value>> { value.as_array().ok_or_else(|| anyhow!("expected array")) }
fn number_i64(value: &Value) -> Result<i64> { value.as_i64().ok_or_else(|| anyhow!("expected signed integer")) }
fn number_u64(value: &Value) -> Result<u64> { value.as_u64().ok_or_else(|| anyhow!("expected unsigned integer")) }
fn single_char(value: &Value) -> Result<char> {
    let text = value.as_str().ok_or_else(|| anyhow!("expected character string"))?;
    let mut chars = text.chars();
    let result = chars.next().ok_or_else(|| anyhow!("character cannot be empty"))?;
    if chars.next().is_some() { bail!("expected one character") }
    Ok(result)
}

fn wasmtime_error(error: wasmtime::Error) -> anyhow::Error { anyhow!(error.to_string()) }
