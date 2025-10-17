# GoLinux

GoLinux is a Linux command execution tool written in Go that supports both local CLI and remote gRPC execution. It allows users to execute Linux commands and manage job lifecycle with SQLite persistence and real-time monitoring.

## Architecture

The project consists of several main components:

1. **Job** - Defines the data structure and state management for command execution tasks
2. **Worker** - Responsible for executing Linux commands with event-driven architecture
3. **JobManager** - Manages job creation, execution, persistence, and lifecycle with real-time event processing
4. **gRPC Server** - Provides remote API for job management with channel-based streaming
5. **gRPC Client** - Command-line interface for remote job execution

## Features

- ✅ **Event-Driven Architecture** - Modern channel-based event system for real-time job monitoring
- ✅ **Real-time Streaming Output** - Live output streaming as commands execute with zero-latency
- ✅ **Local CLI Execution** - Direct command execution via CLI
- ✅ **Remote gRPC API** - Execute jobs remotely via gRPC with streaming support
- ✅ **Smart Command Parsing** - Intelligent parsing for shell commands (bash -c, sh -c, etc.)
- ✅ **SQLite Persistence** - Jobs stored in SQLite database with full state and indexing
- ✅ **Process Management** - Start, stop, and monitor processes with PID tracking
- ✅ **High Performance** - Channel-based communication eliminates callback overhead
- ✅ **Multi-threading Safe** - Concurrent job execution with proper locking
- ✅ **Error Handling** - Comprehensive error capture and reporting
- ✅ **Memory Optimization** - Caching of active jobs only

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

# Real-time streaming output examples
./bin/golinux-client run bash -c "echo 'Starting...'; sleep 3; echo 'Done!'"
./bin/golinux-client run "for i in {1..5}; do echo \"Step \$i\"; sleep 1; done"
```

## Building

### Prerequisites

- Go 1.23+
- SQLite3 (included with go-sqlite3 driver)
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

## Configuration

### CLI Options

**golinux** (local CLI):
- No additional configuration needed

**golinux-server** (gRPC server):
- `-port`: Server port (default: 50051)
- `-max-jobs`: Maximum concurrent jobs (default: 1000)
- `-db-path`: SQLite database path (default: jobs.db)

**golinux-client** (gRPC client):
- `-server`: Server address (default: localhost:50051)  
- `-timeout`: Command timeout in seconds (default: 60)

### Job Storage

Jobs are automatically persisted in SQLite database with optimized performance:

```
jobs.db                    # SQLite database file
├── jobs table             # Main job records with indexes
│   ├── id (PRIMARY KEY)   # Unique job identifier
│   ├── command            # Command to execute
│   ├── status             # Job status (indexed)
│   ├── created_at         # Creation timestamp (indexed)
│   ├── started_at         # Start timestamp
│   ├── completed_at       # Completion timestamp
│   ├── result             # Command output
│   ├── error_message      # Error information
│   ├── exit_code          # Process exit code
│   ├── timeout            # Timeout setting
│   └── pid                # Process ID
```


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
│   ├── storage/           # Storage layer (SQLite implementation)
│   └── grpc/              # gRPC server implementation
├── proto/                 # Protocol buffer definitions
│   ├── golinux.proto      # Service definition
│   ├── golinux.pb.go      # Generated message types
│   └── golinux_grpc.pb.go # Generated service code
├── docs/                  # Documentation
├── jobs.db                # SQLite database (created at runtime)
├── bin/                   # Compiled binaries
├── Makefile               # Build automation
└── README.md              # This file
```

## Development

### Adding New Features

1. **Extend the Job struct** in `pkg/job/job.go`
2. **Add new event types** in `pkg/worker/worker.go` if needed
3. **Update the Worker** in `pkg/worker/worker.go` for execution logic
4. **Modify JobManager** in `pkg/manager/manager.go` for event handling
5. **Update Storage interface** in `pkg/storage/interface.go` if needed
6. **Update SQLite implementation** in `pkg/storage/sqlite.go` for new fields
7. **Update protobuf** in `proto/golinux.proto` for gRPC API changes
8. **Regenerate proto code** with `make proto`

### Testing

```bash
# Run tests
make test

# Test gRPC functionality
./bin/golinux-server &
./bin/golinux-client run echo "test"

# Test complex shell commands
./bin/golinux-client run bash -c "for i in {1..3}; do echo \"Test \$i\"; sleep 1; done"

pkill golinux-server
```
