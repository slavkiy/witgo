use anyhow::{Result, anyhow, bail};
use serde_json::{Map, Value, json};
use std::sync::{Arc, Mutex};
use wasmtime::component::{Type, Val};

use crate::handles::{HandleTable, StoredHandle, insert_handle, take_handle};

pub fn json_to_val(value: &Value, ty: &Type, handles: &Arc<Mutex<HandleTable>>) -> Result<Val> {
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
            let object = value.as_object().ok_or_else(|| anyhow!("expected record object"))?;
            Val::Record(
                record
                    .fields()
                    .map(|field| {
                        let value = object
                            .get(field.name)
                            .ok_or_else(|| anyhow!("record field {:?} is missing", field.name))?;
                        Ok((field.name.to_owned(), json_to_val(value, &field.ty, handles)?))
                    })
                    .collect::<Result<_>>()?,
            )
        }
        Type::Tuple(tuple) => {
            let values = array(value)?;
            let types = tuple.types().collect::<Vec<_>>();
            if values.len() != types.len() {
                bail!("tuple has {} values, expected {}", values.len(), types.len())
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
            let object = value.as_object().ok_or_else(|| anyhow!("expected variant object"))?;
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
                    object.get("value").ok_or_else(|| anyhow!("variant.value is required"))?,
                    &ty,
                    handles,
                )?)),
                None => None,
            };
            Val::Variant(case.to_owned(), payload)
        }
        Type::Enum(enumeration) => {
            let name = value.as_str().ok_or_else(|| anyhow!("expected enum string"))?;
            if !enumeration.names().any(|item| item == name) {
                bail!("unknown enum case {name:?}")
            }
            Val::Enum(name.to_owned())
        }
        Type::Option(option) => {
            let object = value.as_object().ok_or_else(|| anyhow!("expected option object"))?;
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
            let object = value.as_object().ok_or_else(|| anyhow!("expected result object"))?;
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
                .map(|v| v.as_str().map(str::to_owned).ok_or_else(|| anyhow!("flag must be a string")))
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

pub fn val_to_json(value: &Val, handles: &Arc<Mutex<HandleTable>>) -> Result<Value> {
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
                .map(|(k, v)| Ok(Value::Array(vec![val_to_json(k, handles)?, val_to_json(v, handles)?])))
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
        Val::Resource(resource) => insert_handle(handles, "resource", resource.owned(), StoredHandle::Resource(*resource)),
        Val::Future(future) => insert_handle(handles, "future", false, StoredHandle::Future(future.clone())),
        Val::Stream(stream) => insert_handle(handles, "stream", false, StoredHandle::Stream(stream.clone())),
        Val::ErrorContext(_) => insert_handle(handles, "error-context", false, StoredHandle::ErrorContext(value.clone())),
    })
}

fn array(value: &Value) -> Result<&Vec<Value>> {
    value.as_array().ok_or_else(|| anyhow!("expected array"))
}

fn number_i64(value: &Value) -> Result<i64> {
    value.as_i64().ok_or_else(|| anyhow!("expected signed integer"))
}

fn number_u64(value: &Value) -> Result<u64> {
    value.as_u64().ok_or_else(|| anyhow!("expected unsigned integer"))
}

fn single_char(value: &Value) -> Result<char> {
    let text = value.as_str().ok_or_else(|| anyhow!("expected character string"))?;
    let mut chars = text.chars();
    let result = chars.next().ok_or_else(|| anyhow!("character cannot be empty"))?;
    if chars.next().is_some() {
        bail!("expected one character")
    }
    Ok(result)
}
