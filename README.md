# wrtp (Wayland Record To Play)

![Go](https://github.com/jason9075/wrtp/actions/workflows/go.yml/badge.svg)

`wrtp` is a lightweight, daemon-free input automation tool designed specifically for Wayland. It allows you to record mouse and keyboard actions and replay them with resolution independence.

## Features
- **Toggle-based Recording**: Run it once to start recording, run it again to stop.
- **Resolution Independent**: Uses normalized coordinates so your scripts work on any screen.
- **Daemon-free**: No background processes or systemd services required.
- **Wayland Native**: Built for modern Linux environments (NixOS, Hyprland, Sway).

## Prerequisites (NixOS / Wayland)
Because `wrtp` interacts with kernel-level input devices, you need to grant permissions to your user.

### 1. Add Udev Rules
Add the following to your `configuration.nix`:
```nix
services.udev.extraRules = ''
  KERNEL=="uinput", GROUP="input", MODE="0660"
  KERNEL=="event*", GROUP="input", MODE="0660"
'';
```

## Usage
`wrtp` requires `sudo` because it interacts directly with kernel input devices via `libinput`.

### 1. Test Mode (Quick Start)
Record for 5 seconds and play back immediately once.
```bash
sudo make test-mode
# or
sudo go run cmd/wrtp/main.go --test
```

### 2. Manual Toggle Mode
Use this for longer recordings. Run the command once to start, and again to stop.
```bash
# Start recording
sudo go run cmd/wrtp/main.go

# ... perform your mouse/keyboard actions ...

# Stop recording (run in another terminal or press Ctrl+C)
sudo go run cmd/wrtp/main.go
```

### 3. Manual Replay
Replay the latest recording directly.
```bash
./bin/wrtp --play
```

## CLI Flags
- `--test <seconds>`: Record for N seconds (e.g., `--test 10`) and play back once.
- `--play`: Replay the latest recording.
- `--help`: Show the usage message.

## Overlay UI
When recording, a small window with a **Red Dot** and **"REC"** text will appear on your screen to indicate that input is being captured. This window will close automatically when recording stops.

## Files
- `/tmp/wrtp.lock`: PID file used to track the recording state.
- `/tmp/wrtp.yml`: The recorded input events file.

## Build and Run
You can use the `Makefile` to automate common tasks:

- **Build**: `make build`
- **Run**: `make run`
- **Clean**: `make clean`
- **Help**: `make help`

## License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
