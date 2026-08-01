use std::collections::BTreeMap;
use wasmtime::Engine;
use wasmtime::component::types::{ComponentFunc, ComponentItem};
use wasmtime::component::{Component, Type};

pub fn component_functions(
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

fn function_signature(function: &ComponentFunc) -> String {
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

#[cfg(test)]
mod tests {
    use anyhow::Result;
    use wasmtime::Config;
    use wasmtime::Engine;
    use wasmtime::component::Component;

    use super::component_functions;

    #[test]
    fn component_ping_reports_sorted_function_names() -> Result<()> {
        let mut config = Config::new();
        config.wasm_component_model(true);
        config.wasm_component_model_map(true);
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
