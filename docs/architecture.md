# Runtime architecture

`witgo` runs WebAssembly Components in-process through a bundled Wasmtime
shared library. There is no child executable, stdin/stdout IPC, network
download, or separate runtime installation.

The Go package embeds one gzip-compressed library per supported target:

- Windows amd64/arm64: `witgo_bridge.dll`;
- Linux amd64/arm64: `libwitgo_bridge.so`;
- macOS amd64/arm64: `libwitgo_bridge.dylib`.

On first use the current platform library is decompressed into the user cache.
Operating systems cannot load a DLL/shared object directly from a Go byte
slice, so a cache file is unavoidable. Its filename contains the content hash,
installation is atomic, and the compiled-in SHA-256 is checked both before
installation and on every cache hit. No new process is started.

`BridgePath` (or `WITGO_COMPONENT_LIBRARY`) can select an administrator-managed
library; `BridgeSHA256` pins it. `DisableEmbeddedBridge` forbids materializing
the bundled copy.

## In-process protocol

Go calls a small stable C ABI through `purego` (no CGO). Inside Rust, the
existing JSON value protocol travels over in-memory channels. Initialization
performs a strict version and feature handshake containing `protocol_version`,
`witgo_version`, `bridge_version`, `wasmtime_version`, and `features`. Before
instantiation, the bridge answers a contract `ping` with sorted component
import/export function names; generated Go bindings compare these names with
both the WIT manifest and registered host adapters before sending `start`.
The Go side sends its required bridge version and feature set in `init`; Rust
rejects either mismatch, while Go independently validates the versions and
features returned in `pong`. There is no compatibility fallback.

Calls on a Runtime are serialized; separate Runtime values may execute
concurrently. `Close` drops the Wasmtime state, joins its internal worker
thread, unloads the library when possible, and is idempotent. Wasm execution
timeouts use Wasmtime epoch interruption. An arbitrary deadlock inside native
library code cannot safely be killed without
terminating the Go process; this is the tradeoff for in-process execution.

## Supported values

The codec implements booleans, all WIT integers and floats, `char`, strings,
lists, records, tuples, variants, enums, options, results and flags.
Resources, futures, streams and `error-context` use opaque runtime-bound handle
tokens. The bridge retains the corresponding Wasmtime value, validates its
kind and Store, transfers `own` values, preserves `borrow` values for the call,
and explicitly drops/closes retained handles.

Resource types are also included in the pre-instantiation contract manifest.
The dynamic API can pass future/stream handles between calls and close them,
but it cannot read or write an arbitrary typed payload. Payload consumption
requires generated Rust specialization because Wasmtime's type-erased
`FutureAny` and `StreamAny` only expose conversion to statically typed readers.

## Security boundary

The shared library remains native trusted code. Release preparation builds all
six libraries from `Cargo.lock`, tests each library through Go, publishes raw
and compressed SHA-256 checksums, SPDX SBOMs and GitHub attestations, then
commits the exact compressed libraries into module source before a tag can be
created. Windows Authenticode and Apple signing/notarization remain required
production gates for minimizing antivirus false positives.
