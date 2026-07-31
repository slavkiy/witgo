# Releasing native libraries

Run the `bridge-binaries` workflow manually on the release branch first. Its
native matrix builds and tests Windows, Linux and macOS shared libraries for
amd64 and arm64, then commits the compressed libraries and checksums under
`internal/bridgebin/bin`. Review that bot commit and only then create the tag.

On a tag the same matrix rebuilds everything and fails if the results differ
from the libraries already contained in module source. This guarantees that a
normal `go get github.com/slavkiy/witgo@<tag>` contains its runtime and never
needs a separate download.

Each build also produces an SPDX JSON SBOM and GitHub/Sigstore provenance
attestation. Production Windows libraries must be Authenticode-signed and
timestamped; macOS libraries must use Developer ID and notarization before
checksums are generated. Signing keys belong in CI key management, never in
the repository.

Native local build:

```text
cd bridge
cargo build --locked --release --lib
```

The output is `witgo_bridge.dll`, `libwitgo_bridge.so`, or
`libwitgo_bridge.dylib`. The Go packager creates deterministic gzip data plus
checksums for both raw and compressed forms.
