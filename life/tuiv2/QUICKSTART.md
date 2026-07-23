# Quick Start

## Run TUI v2

### 1. Start Simulation
```bash
go run main.go
# Wait for: 🚀 Simulation auto-started!
```

### 2. Start TUI (new terminal)
```bash
./tuiv2_bin
```

## Troubleshooting

**Connection refused**: Simulation not running. Start it first.

**Stale socket**: `rm /tmp/habitat_mesh.sock` and restart simulation.

**Check if running**: `lsof /tmp/habitat_mesh.sock`
