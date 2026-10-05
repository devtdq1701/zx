# zx

**zx** is a fast, standalone Zabbix CLI and interactive REPL client written in Go. It supports multi-cluster profiles, high-concurrency metric querying, combined multi-host PNG graph rendering, web scenario management, and a universal JSON-RPC API fallback.

---

## Features

- **Multi-Profile Management**: Seamlessly switch between Zabbix environments (`zx context use prod`). Credentials are kept secure (never leaked to terminal history or output).
- **High Performance**: Concurrent metric collection and trend queries across multiple hosts with automatic session token caching (~80ms query times).
- **Graph Export**: Render and download combined multi-metric PNG graphs directly from Zabbix web frontend.
- **Web Scenario Management (`httptest`)**: List, inspect, create, and delete web scenarios directly via CLI.
- **Raw History & Logs (`history`)**: Dump raw data points and event logs with automatic value type detection and timezone formatting.
- **Universal JSON-RPC Fallback (`api`)**: Invoke arbitrary Zabbix API methods with active credentials injected automatically.
- **Safety by Default**: All mutations (`create`, `update`, `delete`, `import`) run in **dry-run** mode by default and require `--yes` to execute.
- **Interactive REPL**: Launch an interactive terminal session simply by running `zx`.

---

## Installation

### From Source
```bash
git clone https://github.com/devtdq1701/zx.git
cd zx
make build
# Binary is generated at bin/zx
cp bin/zx ~/.local/bin/
```

### Go Install
```bash
go install zx/cmd/zx@latest
```

---

## Quick Start

### 1. Configure Profiles
```bash
# Add a profile using an API token (Zabbix 6.4 / 7.x)
zx profile add prod --url https://zabbix.example.com --token "YOUR_API_TOKEN"

# Or using username/password
zx profile add staging --url https://zabbix-dev.example.com --user admin --password secret

# Check connection
zx preflight
```

### 2. View Hosts & Metrics
```bash
# Inspect hosts by IP, hostname, or pattern
zx show_hosts 192.168.1.10

# High-level CPU, RAM, and Load statistics
zx show_host_stats host1,host2 -d 7 --business-hours

# Export combined PNG graph
zx export_graph host1,host2 -m cpu,ram -d 7 -o cluster_stats.png
```

### 3. Web Scenario & URL Probing
```bash
# List all configured web scenarios
zx httptest list

# Create a web check (dry-run preview first)
zx httptest create --host host01 --name "Login Check" --url "http://host01:8080/login"

# Confirm execution
zx httptest create --host host01 --name "Login Check" --url "http://host01:8080/login" --yes
```

### 4. Raw History & Logs
```bash
# Query the latest 10 values for an item (auto-detects type)
zx history get 70746 --limit 10

# Query history for a specific timeframe
zx history get 70746 --since "now-2h" --until "now" --format json
```

### 5. Universal JSON-RPC API Fallback
Call any Zabbix method directly without `curl`:
```bash
# Read-only methods execute directly
zx api apiinfo.version
zx api history.get '{"itemids":["70746"],"limit":5}'

# Mutation methods require --yes
zx api host.delete '["10001"]' --yes
```

---

## Configuration

Profiles and session tokens are stored in:
- Configuration: `~/.config/zx/config.yaml` (file mode `0600`)
- Session caches: `~/.config/zx/sessions/<profile>.json`

---

## License

MIT License. See [LICENSE](LICENSE) for details.
