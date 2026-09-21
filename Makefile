APP_NAME    := eworkspace-server
BIN_DIR     := bin
MAIN_PKG    := ./cmd/server
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)

export GOPROXY ?= https://goproxy.cn,direct

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: vet
vet:
	go vet ./...

.PHONY: run
run:
	go run $(MAIN_PKG) -config configs/config.yaml

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) $(MAIN_PKG)

.PHONY: build-linux
build-linux:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" \
		-o $(BIN_DIR)/$(APP_NAME)-linux-amd64 $(MAIN_PKG)

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)
