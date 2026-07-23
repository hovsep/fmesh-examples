# TUI v2 - Modern Terminal Dashboard

Modern, modular terminal dashboard for the Life simulation, built with Bubble Tea.

## Features

- 4-quadrant dashboard (Cardiovascular, Respiratory, Nervous, Gas Exchange)
- Real-time sparklines for each vital sign
- Health indicators (✓/⚠/✗)
- Tab navigation between views
- 14 vital signs updating in real-time

## Quick Start

### 1. Start Simulation

```bash
go run main.go
```

Wait for:
```
🚀 Simulation auto-started!
```

### 2. Start TUI v2 (in new terminal)

```bash
./tuiv2_bin
```

Or build from source:
```bash
go build -o tuiv2_bin ./tuiv2
./tuiv2_bin
```

## Controls

- **Tab** / **Shift+Tab**: Switch views
- **1-4**: Jump to view
- **q**: Quit

## Displayed Signals (14)

**Cardiovascular**: Heart Rate, Blood O₂, Blood CO₂
**Respiratory**: Resp Rate, Pleural Pressure, Lung Volumes (L/R)
**Nervous**: Brain Activity, Brain Trend, Body Temp
**Gas Exchange**: Lung Flows (L/R), Gas Composition
