The Brunson for a simple local setup. Inspired by the simplicity of Hivemind.

## Installation

```bash
go build -o pguard .
cp pguard ~/bin/pguard
chmod +x ~/bin/pguard
```

## Configuration

Create a config file at one of these locations (searched in order):
- `./pguard.yaml` (current directory)
- `$XDG_CONFIG_HOME/pguard/pguard.yaml` (typically `~/.config/pguard/pguard.yaml`)
- `~/.pguard.yaml`

## Usage

Run with auto-discovery:
```bash
pguard
```

Or specify a config file:
```bash
pguard --config path/to/pguard.yaml
```
