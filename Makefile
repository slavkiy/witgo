.PHONY: help test vet lint fmt bridge-test bridge-build witgen-test ci

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

ci: test vet witgen-test bridge-test lint
