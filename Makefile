.PHONY: proto test check-tinygo build check-wasm-size verify clean

GO_TEXT_WASM := go/examples/openai-chat-compatible/driver.wasm
GO_IMAGE_WASM := go/examples/openai-image-compatible/driver.wasm
RUST_TEXT_WASM := rust/target/wasm32-unknown-unknown/release/openai_chat_compatible.wasm
RUST_IMAGE_WASM := rust/target/wasm32-unknown-unknown/release/openai_image_compatible.wasm
WASM_MODULES := $(GO_TEXT_WASM) $(GO_IMAGE_WASM) $(RUST_TEXT_WASM) $(RUST_IMAGE_WASM)
TINYGO ?= tinygo
TINYGO_VERSION := 0.39.0

proto:
	protoc --proto_path=. --descriptor_set_out=/dev/null abi/v1/driver.proto
	cmp -s abi/v1/driver.proto go/driver/testdata/abi/v1/driver.proto
	cmp -s conformance/v1/wire_vectors.json go/driver/testdata/conformance/v1/wire_vectors.json
	cmp -s conformance/v1/usage_validation.json go/driver/testdata/conformance/v1/usage_validation.json
	cmp -s conformance/v1/outcome_validation.json go/driver/testdata/conformance/v1/outcome_validation.json

test: proto
	cd go && go test ./... -count=1 && go vet ./...
	cd go && go test -tags=tinygo ./driver -count=1
	cd rust && ../scripts/run-rust-cargo.sh fmt --all --check
	cd rust && ../scripts/run-rust-cargo.sh test --workspace
	cd rust && ../scripts/run-rust-cargo.sh clippy --workspace --all-targets -- -D warnings

check-tinygo:
	@actual_version="$$($(TINYGO) version | awk '{print $$3}')"; \
		test "$$actual_version" = "$(TINYGO_VERSION)" || { \
			echo "TinyGo $$actual_version is unsupported; want $(TINYGO_VERSION)" >&2; \
			exit 1; \
		}

build: check-tinygo
	cd go && $(TINYGO) build -tags=tinygo -target=wasm-unknown -no-debug -o examples/openai-chat-compatible/driver.wasm ./examples/openai-chat-compatible
	cd go && $(TINYGO) build -tags=tinygo -target=wasm-unknown -no-debug -o examples/openai-image-compatible/driver.wasm ./examples/openai-image-compatible
	./scripts/normalize-tinygo-wasm.sh $(GO_TEXT_WASM)
	./scripts/normalize-tinygo-wasm.sh $(GO_IMAGE_WASM)
	cd rust && ../scripts/run-rust-cargo.sh build --release --target wasm32-unknown-unknown -p openai-chat-compatible -p openai-image-compatible

check-wasm-size:
	@for module in $(WASM_MODULES); do \
		bytes="$$(wc -c <"$$module" | tr -d '[:space:]')"; \
		if [ "$$bytes" -gt 1048576 ]; then \
			echo "warning: $$module is $$bytes bytes; soft budget is 1048576 bytes" >&2; \
		else \
			echo "$$module: $$bytes bytes (within 1 MiB soft budget)"; \
		fi; \
	done

verify: build
	$(MAKE) check-wasm-size
	./scripts/verify-wasm.sh $(GO_TEXT_WASM)
	./scripts/verify-wasm.sh $(GO_IMAGE_WASM)
	./scripts/verify-wasm.sh $(RUST_TEXT_WASM)
	./scripts/verify-wasm.sh $(RUST_IMAGE_WASM)

clean:
	find go/examples -type f -name driver.wasm -delete
	cd rust && ../scripts/run-rust-cargo.sh clean
