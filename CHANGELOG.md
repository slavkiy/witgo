# Changelog

## v0.2.0

- Added strict Go/Rust bridge version and feature negotiation.
- Added contract inspection, validation, generated ping manifests, and typed
  structural signatures.
- Added live runtime-bound handles for WIT resources, futures, streams, and
  error contexts.
- Added enum, flags, variant, resource, and nested-list coverage to generated
  bindings and end-to-end tests.
- Added type-safe Option, Result, Char, Tuple0...Tuple16, generated enum/flags/
  variant helpers, and collision-safe generated client locals.
- Added CI coverage for Linux, macOS, and Windows on amd64 and arm64.
- Removed the pre-Component-Model API and deprecated CLI flags.

This release requires bridge protocol 3 and bridge version 0.2.0. Native
libraries from older releases are rejected during startup.
