GO          ?= go
GOFLAGS     ?=
COVERFILE   ?= cover.out
COVER_MIN   ?= 80.0
BUILD_DIR   ?= build

.PHONY: build test cover lint smoketest tidy clean help

help:
	@echo "Targets:"
	@echo "  build      go build ./..."
	@echo "  test       go test -race ./..."
	@echo "  cover      go test with $(COVER_MIN)% coverage gate"
	@echo "  lint       gofmt + go vet"
	@echo "  smoketest  run cmd/smoketest against ./smoke-test.json"
	@echo "  tidy       go mod tidy"
	@echo "  clean      remove build artifacts"

build:
	$(GO) build ./...

test:
	$(GO) test -race ./...

cover:
	$(GO) test -race \
		-coverpkg=github.com/maviance/smobilpay-go,github.com/maviance/smobilpay-go/internal/... \
		-coverprofile=$(COVERFILE) \
		github.com/maviance/smobilpay-go github.com/maviance/smobilpay-go/internal/...
	@total=$$($(GO) tool cover -func=$(COVERFILE) | awk '/^total:/ { print $$3 }' | tr -d '%'); \
	awk -v t=$$total -v m=$(COVER_MIN) 'BEGIN { \
		if (t+0 < m+0) { printf "coverage %.1f%% below %.1f%% gate\n", t, m; exit 1 } \
		else { printf "coverage %.1f%% (gate %.1f%%) OK\n", t, m } }'

lint:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "gofmt issues:"; echo "$$out"; exit 1; fi
	$(GO) vet ./...

smoketest:
	$(GO) run ./cmd/smoketest

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BUILD_DIR) $(COVERFILE) coverage.html
