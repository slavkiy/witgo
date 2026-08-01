.PHONY: help test vet lint fmt bridge-test bridge-build witgen-test wasm-role-test tinygo-test tinygo-e2e ci

help:
	@printf '%s\n' \
	'Available targets:' \
	'  make test         - go test ./...' \
	'  make vet          - go vet ./...' \
	'  make lint         - cargo fmt --check && cargo clippy -- -D warnings' \
	'  make fmt          - gofmt and cargo fmt' \
	'  make bridge-test  - cargo test --locked for bridge' \
	'  make bridge-build - cargo build --locked --lib for bridge' \
	'  make witgen-test  - go test ./... in cmd/witgen' \
	'  make wasm-role-test - compile witgo as a WASM plugin' \
	'  make tinygo-test  - compile all packages with TinyGo' \
	'  make tinygo-e2e   - run TinyGo through the CGo bridge backend' \
	'  make ci           - local verification bundle'

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(git ls-files '*.go')
	cd bridge && cargo fmt

lint:
	cd bridge && cargo fmt --check
	cd bridge && cargo clippy --locked -- -D warnings

bridge-test:
	cd bridge && cargo test --locked

bridge-build:
	cd bridge && cargo build --locked --lib

witgen-test:
	cd cmd/witgen && go test ./...

wasm-role-test:
	GOOS=wasip1 GOARCH=wasm CGO_ENABLED=0 go test -c -o .witgo-wasm-role.test .

tinygo-test:
	tinygo test ./... -run '^$$'

tinygo-e2e: bridge-build
	CGO_ENABLED=1 WITGO_COMPONENT_LIBRARY=bridge/target/debug/libwitgo_bridge.so WITGO_DISABLE_EMBEDDED_BRIDGE=1 tinygo test . -run '^(TestTinyGoUsesCGoBridgeBackend|TestComponentVersionHandshake)$$'

ci: test vet witgen-test wasm-role-test tinygo-test bridge-test lint
