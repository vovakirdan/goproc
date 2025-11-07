.PHONY: build fmt format proto

GO_BIN_DIR := $(shell go env GOBIN)
ifeq ($(GO_BIN_DIR),)
GO_BIN_DIR := $(shell go env GOPATH)/bin
endif
PROTOC_GEN_GO := $(GO_BIN_DIR)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(GO_BIN_DIR)/protoc-gen-go-grpc

# Сборка бинарного файла
build:
	go build -o goproc ./cmd/goproc

# Форматирование кода
fmt:
	go fmt ./...

# Алиас для fmt
format: fmt

# Генерация protobuf кода
proto:
	@test -x $(PROTOC_GEN_GO) || (echo "protoc-gen-go not found. Run 'go install google.golang.org/protobuf/cmd/protoc-gen-go@latest'" && exit 1)
	@test -x $(PROTOC_GEN_GO_GRPC) || (echo "protoc-gen-go-grpc not found. Run 'go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest'" && exit 1)
	protoc --plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
	       --plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) \
	       --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       api/proto/goproc/v1/goproc.proto

