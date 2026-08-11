# Dashboard — integrated terminal UI

The Life simulation's front end, built with Bubble Tea. The dashboard and the
command line live in **one** program with the simulation: the tabbed views are on
top, a command prompt is pinned to the bottom, and telemetry flows over an
in-process channel — there is no socket and no second process to start.

## Run

```bash
go run .        # from the simulation/life/ directory
```

The simulation auto-starts and the dashboard opens. Type commands at the `›`
prompt (try `help`).

Piping a script in (non-interactive stdin) runs headless with a plain prompt and
no dashboard:

```bash
printf 'activity:start 3\ntime:now\nexit\n' | go run . --plain
```

## Controls

Typing always goes to the command line, so navigation uses modifiers and the mouse:

- **type + Enter**: run a command · **Tab**: complete a command name
- **↑ / ↓**: command history · **PgUp / PgDn**: scroll the transcript
- **Ctrl+← / Ctrl+→** or **Alt+1‑7** or **mouse click**: switch tabs
- **Alt+s**: split vs overlaid lungs (Respiratory view)
- **Ctrl+↑ / Ctrl+↓**: faster / slower repaint
- **exit** or **Ctrl+C**: quit

## Views

Cardiovascular (ECG + gases), Respiratory (breathing waveforms), Nervous,
Metabolic, Affect, Body, and an Overview. Most screens are generated from the
telemetry catalog, so a new metric appears without any UI code changing.
