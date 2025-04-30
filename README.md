# GoLinux

GoLinux is a simple Linux command execution tool written in Go. It allows users to execute Linux commands via command line and view the results.

The project consists of three main components:

1. **Job** - Defines the data structure and state management for command execution tasks
2. **Worker** - Responsible for executing Linux commands
3. **JobManager** - Manages the creation and execution of Jobs

### Building

```bash
make build
```

### Commands

1. Execute command:
   ```bash
   ./golinux run <command>
   
   # Examples
   ./golinux run ls -la
   ./golinux run echo "Hello, World!"
   ```

2. Get help:
   ```bash
   ./golinux help
   ```