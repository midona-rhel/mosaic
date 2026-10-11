GO ?= go
PYTHON ?= python3

.PHONY: build run demo test check preview capture smoke clean
build:
	$(GO) build -trimpath -o bin/mosaic ./cmd/mosaic
run:
	$(GO) run ./cmd/mosaic
demo:
	$(GO) run ./cmd/mosaic --demo
test:
	$(GO) test -race ./...
check: test
	$(GO) vet ./...
preview: build
	$(PYTHON) scripts/render_preview.py
capture: build
	$(PYTHON) scripts/capture_interactions.py
smoke: build
	$(PYTHON) scripts/smoke_pty.py
clean:
	rm -rf bin dist
