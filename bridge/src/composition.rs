use anyhow::{Context, Result, anyhow, bail};
use serde::Deserialize;
use std::collections::BTreeMap;
use std::fs;
use wac_graph::{CompositionGraph, EncodeOptions, NodeId, PackageId, types::Package};

#[derive(Clone, Deserialize)]
pub struct CompositionPlug {
    pub interface: String,
    pub component: String,
    #[serde(default)]
    pub dependencies: Vec<CompositionPlug>,
}

pub fn compose_components(root: &str, dependencies: &[CompositionPlug]) -> Result<Vec<u8>> {
    let mut graph = CompositionGraph::new();
    let mut counter = 0_u64;
    let mut instances = BTreeMap::new();
    let (root_package, root_instance) =
        instantiate_composition_node(&mut graph, root, dependencies, &mut counter, &mut instances)?;

    let exports = graph.types()[graph[root_package].ty()]
        .exports
        .keys()
        .cloned()
        .collect::<Vec<_>>();
    for name in exports {
        let export = graph
            .alias_instance_export(root_instance, &name)
            .with_context(|| format!("alias root export {name:?}"))?;
        graph
            .export(export, &name)
            .with_context(|| format!("export root item {name:?}"))?;
    }

    graph
        .encode(EncodeOptions::default())
        .context("encode same-store component composition")
}

fn instantiate_composition_node(
    graph: &mut CompositionGraph,
    component: &str,
    dependencies: &[CompositionPlug],
    counter: &mut u64,
    instances: &mut BTreeMap<String, (PackageId, NodeId)>,
) -> Result<(PackageId, NodeId)> {
    let key = composition_node_key(component, dependencies);
    if let Some(node) = instances.get(&key) {
        return Ok(*node);
    }
    *counter = counter
        .checked_add(1)
        .ok_or_else(|| anyhow!("composition node counter overflow"))?;
    let package_name = format!("witgo:node{counter}");
    let source =
        fs::read(component).with_context(|| format!("read composition component {component:?}"))?;
    let bytes = wat::parse_bytes(&source)
        .with_context(|| format!("parse composition component {component:?}"))?
        .into_owned();
    let package = Package::from_bytes(&package_name, None, bytes, graph.types_mut())
        .with_context(|| format!("decode composition component {component:?}"))?;
    let package_id = graph
        .register_package(package)
        .with_context(|| format!("register composition component {component:?}"))?;
    let instance = graph.instantiate(package_id);

    let mut interfaces = BTreeMap::new();
    for dependency in dependencies {
        if dependency.interface.is_empty() || dependency.component.is_empty() {
            bail!("composition interface and component path must not be empty")
        }
        if interfaces
            .insert(dependency.interface.clone(), ())
            .is_some()
        {
            bail!(
                "multiple composition providers selected for exact interface {:?}",
                dependency.interface
            )
        }
        let (_, provider_instance) = instantiate_composition_node(
            graph,
            &dependency.component,
            &dependency.dependencies,
            counter,
            instances,
        )?;
        let provider_export = graph
            .alias_instance_export(provider_instance, &dependency.interface)
            .with_context(|| {
                format!(
                    "component {:?} does not export exact interface {:?}",
                    dependency.component, dependency.interface
                )
            })?;
        graph
            .set_instantiation_argument(instance, &dependency.interface, provider_export)
            .with_context(|| {
                format!(
                    "interface {:?} from {:?} is incompatible with consumer {:?}",
                    dependency.interface, dependency.component, component
                )
            })?;
    }

    instances.insert(key, (package_id, instance));
    Ok((package_id, instance))
}

fn composition_node_key(component: &str, dependencies: &[CompositionPlug]) -> String {
    let mut key = String::from(component);
    for dependency in dependencies {
        key.push('\0');
        key.push_str(&dependency.interface);
        key.push('\0');
        key.push_str(&composition_node_key(
            &dependency.component,
            &dependency.dependencies,
        ));
    }
    key
}
