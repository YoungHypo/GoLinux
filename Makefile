.PHONY: all build clean run test deps proto server client

all: proto build

GOPATH:=$(shell go env GOPATH)
CURRENT_DIR:=$(shell pwd)

OUTPUT := bin/golinux
SERVER_OUTPUT := bin/golinux-server
CLIENT_OUTPUT := bin/golinux-client
CMD_DIR := cmd
PROTO_DIR := proto

# Generate protobuf code
proto:
	protoc --go_out=. --go-grpc_out=. $(PROTO_DIR)/*.proto

# Build CLI tool
build:
	mkdir -p bin
	go build -o $(OUTPUT) $(CMD_DIR)/main.go

# Build gRPC server
server:
	mkdir -p bin
	go build -o $(SERVER_OUTPUT) $(CMD_DIR)/server/main.go

# Build gRPC client
client:
	mkdir -p bin
	go build -o $(CLIENT_OUTPUT) $(CMD_DIR)/client/main.go

run:
	go run $(CMD_DIR)/main.go

run-server:
	go run $(CMD_DIR)/server/main.go

run-client:
	go run $(CMD_DIR)/client/main.go

clean:
	rm -rf $(OUTPUT) $(SERVER_OUTPUT) $(CLIENT_OUTPUT) bin/ proto/*.pb.go

deps:
	go mod tidy 