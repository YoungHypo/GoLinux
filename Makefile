.PHONY: all build clean run test deps

all: build

GOPATH:=$(shell go env GOPATH)
CURRENT_DIR:=$(shell pwd)

OUTPUT := bin/golinux
CMD_DIR := cmd

build:
	mkdir -p bin
	go build -o $(OUTPUT) $(CMD_DIR)/main.go

run:
	go run $(CMD_DIR)/main.go

clean:
	rm -rf $(OUTPUT) bin/

deps:
	go mod tidy 