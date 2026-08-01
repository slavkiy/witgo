# Релизный процесс witgo

Go module и native bridge релизятся вместе. Релиз считается корректным только
тогда, когда совпадают protocol version, bridge version, feature list и
встроенные native libraries. Актуальный релиз в репозитории сейчас: `v0.2.0`.

Сначала вручную запускается workflow `bridge-binaries` на release-ветке. Он
собирает и тестирует Windows, Linux и macOS shared libraries для `amd64` и
`arm64`, затем коммитит сжатые библиотеки и checksums в
`internal/bridgebin/bin`. Этот bot-коммит нужно проверить до создания тега.

При публикации тега тот же workflow заново собирает всё и падает, если
результаты отличаются от библиотек, уже лежащих в исходниках модуля. Это
гарантирует, что обычный `go get github.com/slavkiy/witgo@<tag>` уже содержит
свой runtime и не требует отдельного download шага.

Каждая сборка также выпускает SPDX JSON SBOM и GitHub/Sigstore provenance
attestation. Production Windows libraries должны быть подписаны
Authenticode-сертификатом и timestamped; macOS libraries должны пройти
Developer ID signing и notarization до генерации checksums. Ключи подписи
должны храниться только в CI key management, а не в репозитории.

## Локальная сборка bridge

```text
cd bridge
cargo build --locked --release --lib
```

Результат: `witgo_bridge.dll`, `libwitgo_bridge.so` или
`libwitgo_bridge.dylib`. Go packager затем создаёт deterministic gzip и
checksums для raw и compressed форм.

## Обязательные проверки перед native matrix

```text
go test ./...
go vet ./...
cd cmd/witgen && go test ./...
cd ../../bridge && cargo fmt --check
cargo test --locked
cargo clippy --locked -- -D warnings
```
