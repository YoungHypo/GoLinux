# GoLinux

GoLinux is a Linux command execution tool written in Go that supports both local CLI and remote gRPC execution. It allows users to execute Linux commands and manage job lifecycle with persistence and real-time monitoring.

## Architecture

The project consists of several main components:

1. **Job** - Defines the data structure and state management for command execution tasks
2. **Worker** - Responsible for executing Linux commands with state callbacks
3. **JobManager** - Manages job creation, execution, persistence, and lifecycle
4. **gRPC Server** - Provides remote API for job management
5. **gRPC Client** - Command-line interface for remote job execution

## Features

- ✅ **Local CLI Execution** - Direct command execution via CLI
- ✅ **Remote gRPC API** - Execute jobs remotely via gRPC
- ✅ **Job Persistence** - Jobs stored as JSON files with full state
- ✅ **Real-time Monitoring** - Stream job status updates
- ✅ **Process Management** - Start, stop, and monitor processes
- ✅ **Multi-threading Safe** - Concurrent job execution with proper locking
- ✅ **Error Handling** - Comprehensive error capture and reporting

## Quick Start

### 1. Local CLI Mode

```bash
# Build the CLI tool
make build

# Execute commands directly
./bin/golinux run ls -la
./bin/golinux run echo "Hello, World!"

# View job status and list
./bin/golinux status <job-id>
./bin/golinux list
./bin/golinux stop <job-id>
./bin/golinux clean
```

### 2. gRPC Server/Client Mode

```bash
# Build all components
make all

# Terminal 1: Start the gRPC server
./bin/golinux-server

# Terminal 2: Use the gRPC client
./bin/golinux-client run ls -la
./bin/golinux-client list
./bin/golinux-client status <job-id>
./bin/golinux-client stream <job-id>  # Real-time updates
./bin/golinux-client stop <job-id>
./bin/golinux-client clean
```

## Building

### Prerequisites

- Go 1.23+
- Protocol Buffers compiler (`protoc`) - for gRPC features
- Make

### Install protobuf tools (for gRPC)

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Build Options

```bash
# Build everything (recommended)
make all

# Build individual components
make build      # CLI tool only
make server     # gRPC server only  
make client     # gRPC client only
make proto      # Generate protobuf code

# Clean build artifacts
make clean
```

## Usage Examples

### Local CLI Examples

```bash
# Simple commands
./bin/golinux run date
./bin/golinux run whoami
./bin/golinux run "ls -la | grep .go"

# Long-running commands
./bin/golinux run sleep 30

# In another terminal, manage the job
./bin/golinux list
./bin/golinux stop <job-id>
```

### gRPC Client Examples

```bash
# Remote execution
./bin/golinux-client -server remote.host:50051 run date

# Custom timeout
./bin/golinux-client -timeout 120 run "sleep 100"

# Stream real-time status
./bin/golinux-client stream <job-id>

# Complex shell commands
./bin/golinux-client run bash -c "echo 'Starting...'; sleep 5; echo 'Done!'"
```

## Configuration

### CLI Options

**golinux** (local CLI):
- No additional configuration needed

**golinux-server** (gRPC server):
- `-port`: Server port (default: 50051)
- `-max-jobs`: Maximum concurrent jobs (default: 1000)

**golinux-client** (gRPC client):
- `-server`: Server address (default: localhost:50051)  
- `-timeout`: Command timeout in seconds (default: 60)

### Job Storage

Jobs are automatically persisted as JSON files in the `jobs/` directory:

```
jobs/
├── 550e8400-e29b-41d4-a716-446655440000.json
├── 6ba7b810-9dad-11d1-80b4-00c04fd430c8.json
└── ...
```

Each file contains complete job information including command, status, timestamps, results, and error messages.

## Documentation

- [gRPC Implementation Guide](docs/grpc.md) - Detailed gRPC usage and API reference
- [Protocol Buffer Definitions](proto/golinux.proto) - Service and message definitions

## Project Structure

```
GoLinux/
├── cmd/                    # Application entry points
│   ├── main.go            # Local CLI application
│   ├── server/main.go     # gRPC server
│   └── client/main.go     # gRPC client
├── pkg/                   # Core packages
│   ├── job/               # Job data structures and methods
│   ├── worker/            # Command execution engine
│   ├── manager/           # Job lifecycle management
│   └── grpc/              # gRPC server implementation
├── proto/                 # Protocol buffer definitions
│   ├── golinux.proto      # Service definition
│   ├── golinux.pb.go      # Generated message types
│   └── golinux_grpc.pb.go # Generated service code
├── docs/                  # Documentation
├── jobs/                  # Job storage (created at runtime)
├── bin/                   # Compiled binaries
├── Makefile               # Build automation
└── README.md              # This file
```

## Development

### Adding New Features

1. **Extend the Job struct** in `pkg/job/job.go`
2. **Update the Worker** in `pkg/worker/worker.go` for execution logic
3. **Modify JobManager** in `pkg/manager/manager.go` for management features
4. **Update protobuf** in `proto/golinux.proto` for gRPC API changes
5. **Regenerate proto code** with `make proto`

### Testing

```bash
# Run tests
make test

# Test gRPC functionality
./bin/golinux-server &
./bin/golinux-client run echo "test"
pkill golinux-server
```
