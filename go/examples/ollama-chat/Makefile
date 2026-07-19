.PHONY: test check-tinygo build verify clean

TINYGO ?= tinygo
TINYGO_VERSION := 0.39.0

test:
	go test ./... -count=1
	go vet ./...

check-tinygo:
	@actual_version="$$($(TINYGO) version | awk '{print $$3}')"; \
		test "$$actual_version" = "$(TINYGO_VERSION)" || { \
			echo "TinyGo $$actual_version is unsupported; want $(TINYGO_VERSION)" >&2; \
			exit 1; \
		}

build: check-tinygo
	$(TINYGO) build -tags=tinygo -target=wasm-unknown -no-debug -o driver.wasm .
	../../../scripts/normalize-tinygo-wasm.sh driver.wasm

verify: build
	../../../scripts/verify-wasm.sh driver.wasm

clean:
	rm -f driver.wasm
