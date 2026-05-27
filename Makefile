GO          ?= go
GOFLAGS     ?=
COVERFILE   ?= cover.out
COVER_MIN   ?= 80.0
JAVA_DIR    ?= ../java
BUILD_DIR   ?= build

.PHONY: build test cover lint smoketest smoketest-compare tidy clean help

help:
	@echo "Targets:"
	@echo "  build              go build ./..."
	@echo "  test               go test -race ./..."
	@echo "  cover              go test with $(COVER_MIN)% coverage gate"
	@echo "  lint               gofmt + go vet"
	@echo "  smoketest          run cmd/smoketest against ./smoke-test.json"
	@echo "  smoketest-compare  diff Go vs Java smoke-test output (JAVA_DIR=$(JAVA_DIR))"
	@echo "  tidy               go mod tidy"
	@echo "  clean              remove build artifacts"

build:
	$(GO) build ./...

test:
	$(GO) test -race ./...

cover:
	$(GO) test -race -coverpkg=./... -coverprofile=$(COVERFILE) ./...
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

smoketest-compare:
	@mkdir -p $(BUILD_DIR)
	@test -f $(JAVA_DIR)/smoke-test.json || \
	    { echo "Error: $(JAVA_DIR)/smoke-test.json not found. Set JAVA_DIR=<path>."; exit 1; }
	$(GO) run ./cmd/smoketest > $(BUILD_DIR)/smoketest.go.txt
	cd $(JAVA_DIR) && ./gradlew runSmokeTest --console=plain \
		--args="$$PWD/smoke-test.json" > $(CURDIR)/$(BUILD_DIR)/smoketest.java.txt 2>&1
	./tools/normalize.sh $(BUILD_DIR)/smoketest.go.txt   > $(BUILD_DIR)/smoketest.go.norm.txt
	./tools/normalize.sh $(BUILD_DIR)/smoketest.java.txt > $(BUILD_DIR)/smoketest.java.norm.txt
	@if diff -u $(BUILD_DIR)/smoketest.java.norm.txt $(BUILD_DIR)/smoketest.go.norm.txt; then \
		echo "Go and Java smoke-test outputs match (modulo redacted volatile fields)."; \
	else exit 1; fi

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BUILD_DIR) $(COVERFILE) coverage.html
