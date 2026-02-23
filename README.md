# Was Geht

[![CI](https://github.com/kylerisse/wasgeht/actions/workflows/ci.yml/badge.svg)](https://github.com/kylerisse/wasgeht/actions/workflows/ci.yml)

## Overview

**Was Geht** is a small Go application that monitors a list of hosts at regular intervals, tracks their availability and metrics, and records the data in Round Robin Databases (RRD). A lightweight web interface serves host status information and interactive graphs of the recorded metrics.

## Features

- **Extensible Check System**: Modular check types via a Registry/Factory pattern. Each check type implements a common `Check` interface and declares its own metrics through a `Descriptor`.
- **Built-in Check Types**:
  - **ping**: ICMP echo requests for host availability and latency.
  - **http**: HTTP/HTTPS endpoint reachability and per-URL response time.
  - **wifi_stations**: Scrapes a Prometheus metrics endpoint for connected WiFi client counts per radio interface.
- **Multi-Metric Checks**: Checks can produce multiple metrics stored as separate data sources in a single RRD file. Multi-metric checks render as stacked area graphs or colored line graphs depending on the check type.
- **Host Status Aggregation**: Each host has an aggregate status (`up`, `down`, `degraded`, `unknown`) computed from all its checks. A check must be alive and have reported within the last 5 minutes to count as healthy.
- **RRD Storage**: Uses Round Robin Databases for time-series data, with configurable archives from 1-minute resolution (1 week) to 8-hour resolution (5 years).
- **Graph Generation**: Generates historical graphs at multiple time scales (15 minutes through 5 years) for each check type on each host.
- **Simple Web Interface**: Serves an HTML/JS front-end to display host status and dynamically loaded graphs. Available in table and flame graph formats.
- **REST API**: Exposes JSON data of all hosts and their status at `GET /api`.
- **Prometheus Support**: Exposes metrics in Prometheus format at `GET /metrics`.

## Requirements

### Using Nix (Recommended)

If you have **direnv** installed, follow the instructions when entering this directory.

If you have **Nix** installed, you can simply enter a development shell with all required dependencies using:

```bash
nix develop
```

You will need experimental features `flakes` and `nix-command`.

This loads the environment specified in `flake.nix`:

- Go (for building),
- gnumake (for Makefile),
- air (for live reload during development),
- rrdtool (for handling RRD databases),
- unixtools.ping (ping utility).

Once inside the shell, you can run the usual make commands

### Without Nix

Ensure the following are installed:

- **Go** (1.25+ recommended)
- **air** (for live reload during development, optional)
- **rrdtool** and **unixtools ping** must be installed and available on the system path.
- Basic Unix tools for building and running (`make`, etc.).

## Quick Start

1. **Clone** the repository:

   ```bash
   git clone https://github.com/kylerisse/wasgeht.git
   cd wasgeht
   ```

2. **Install dependencies**:

   ```bash
   make deps
   ```

3. **Build** the binary:

   ```bash
   make build
   ```

   This will compile the Go code and produce a `wasgehtd` binary in the project root.

4. **Prepare data directories**:

   By default, Was Geht expects two subdirectories under `./data`:
   - `rrds` for storing RRD files
   - `graphs` for storing generated graph images

   These directories will be created automatically if they do not exist, but make sure that `./data` itself exists.

5. **Configure hosts**:

   Update or create your own JSON file listing your hosts (see `sample-hosts.json` for reference).

6. **Run** the application:

   ```bash
   ./wasgehtd --host-file=sample-hosts.json --data-dir=./data --port=1982 --log-level=info
   ```

7. **Access the web interface**:

   Open your browser to [http://localhost:1982](http://localhost:1982). The main table shows all hosts and their current status (UP or DOWN). Hover over the status to see a latency graph.

## Configuration

- **Host File** (`--host-file`): Path to the JSON file specifying host definitions.
- **Data Directory** (`--data-dir`): Root directory that contains `rrds/` and `graphs/`.
- **Port** (`--port`): Port on which the API and front-end are served.
- **Logging Level** (`--log-level`): Set the verbosity of logs (e.g., `debug`, `info`, `warn`, `error`, `fatal`, `panic`).

### Host Configuration

Hosts are defined in a JSON file. Each host can specify an address and a set of checks. Hosts without an explicit `checks` block default to a ping check.

```json
{
	"router": {},
	"google": {
		"address": "8.8.8.8",
		"checks": {
			"ping": { "timeout": "5s" },
			"http": {
				"urls": ["https://www.google.com"]
			}
		}
	},
	"ap1": {
		"checks": {
			"ping": {},
			"wifi_stations": {
				"radios": ["phy0-ap0", "phy1-ap0"]
			}
		}
	},
	"qube": {
		"checks": {
			"ping": {},
			"http": {
				"urls": [
					"http://qube.example.com:2018/sign.json",
					"https://whatsup.example.com",
					"http://mrtg.example.com"
				],
				"timeout": "15s"
			}
		}
	},
	"disabled-example": {
		"checks": {
			"ping": { "enabled": false }
		}
	}
}
```

### Check Types

#### ping

Sends ICMP echo requests to check host availability and measure latency.

| Option    | Type   | Default | Description                    |
| --------- | ------ | ------- | ------------------------------ |
| `timeout` | string | `"3s"`  | Ping timeout (Go duration)     |
| `count`   | number | `1`     | Number of ping packets to send |
| `enabled` | bool   | `true`  | Set to `false` to disable      |

#### http

Performs HTTP GET requests to a list of URLs and reports per-URL response time. Each URL becomes a separate data source in the RRD, rendered as colored lines on the graph. The check succeeds only if all configured URLs return a response (any HTTP status code counts as reachable). Redirects are not followed.

TLS certificate verification is skipped by default to support locally signed certificates.

| Option        | Type     | Default      | Description                        |
| ------------- | -------- | ------------ | ---------------------------------- |
| `urls`        | []string | _(required)_ | List of full URLs to check         |
| `timeout`     | string   | `"10s"`      | HTTP request timeout (Go duration) |
| `skip_verify` | bool     | `true`       | Skip TLS certificate verification  |
| `enabled`     | bool     | `true`       | Set to `false` to disable          |

#### wifi_stations

Scrapes a Prometheus metrics endpoint for `wifi_stations{ifname="..."}` gauge values, reporting connected client counts per radio interface. Each configured radio becomes a separate data source in the RRD, rendered as a stacked area graph.

| Option    | Type     | Default                      | Description                                |
| --------- | -------- | ---------------------------- | ------------------------------------------ |
| `radios`  | []string | _(required)_                 | List of `ifname` label values to monitor   |
| `url`     | string   | `http://{host}:9100/metrics` | Full URL override for the metrics endpoint |
| `timeout` | string   | `"5s"`                       | HTTP scrape timeout (Go duration)          |
| `enabled` | bool     | `true`                       | Set to `false` to disable                  |

The target host expects a Prometheus node exporter (or compatible) exposing metrics like:

```
wifi_stations{ifname="phy0-ap0"} 3
wifi_stations{ifname="phy1-ap0"} 7
```

## Host Status

Each host has an aggregate status derived from all its enabled checks:

| Status       | Color  | Meaning                                                      |
| ------------ | ------ | ------------------------------------------------------------ |
| **up**       | Green  | All checks are alive and reported within the last 5 minutes. |
| **degraded** | Yellow | Some checks are healthy, others are down or stale.           |
| **down**     | Red    | All checks are down (but at least one has reported before).  |
| **unknown**  | Gray   | No checks configured, or no check has ever reported.         |

A check result is considered **stale** if its last successful RRD update is older than 5 minutes. Stale checks are treated the same as down checks for the purpose of host status aggregation.

## API

### `GET /api`

Returns JSON with the status of all hosts:

```json
{
	"google": {
		"address": "8.8.8.8",
		"status": "up",
		"checks": {
			"ping": {
				"alive": true,
				"metrics": {
					"latency_us": 12345
				},
				"lastupdate": 1700000000
			},
			"http": {
				"alive": true,
				"metrics": {
					"https://www.google.com": 45230
				},
				"lastupdate": 1700000000
			}
		}
	},
	"ap1": {
		"status": "up",
		"checks": {
			"ping": {
				"alive": true,
				"metrics": {
					"latency_us": 237
				},
				"lastupdate": 1700000000
			},
			"wifi_stations": {
				"alive": true,
				"metrics": {
					"phy0-ap0": 3,
					"phy1-ap0": 7
				},
				"lastupdate": 1700000000
			}
		}
	},
	"router": {
		"status": "unknown",
		"checks": {}
	}
}
```

The `status` field is one of `up`, `down`, `degraded`, or `unknown` (see [Host Status](#host-status) above).

### `GET /metrics`

Exposes Prometheus-formatted metrics:

```
check_alive{host="google", address="8.8.8.8", check="ping"} 1
check_metric{host="google", address="8.8.8.8", check="ping", metric="latency_us"} 12345
check_alive{host="ap1", address="", check="ping"} 1
check_metric{host="ap1", address="", check="ping", metric="latency_us"} 237
```

## Data Directory Layout

RRD files and graph images are organized into per-host subdirectories:

```
data/
├── rrds/
│   ├── router/
│   │   └── ping.rrd
│   ├── google/
│   │   ├── ping.rrd
│   │   └── http.rrd
│   ├── ap1/
│   │   ├── ping.rrd
│   │   └── wifi_stations.rrd
│   └── ...
└── graphs/
    └── imgs/
        ├── router/
        │   ├── router_ping_15m.png
        │   ├── router_ping_1h.png
        │   └── ...
        ├── google/
        │   ├── google_ping_15m.png
        │   ├── google_http_15m.png
        │   └── ...
        ├── ap1/
        │   ├── ap1_ping_15m.png
        │   ├── ap1_ping_1h.png
        │   ├── ap1_wifi_stations_15m.png
        │   ├── ap1_wifi_stations_1h.png
        │   └── ...
        └── ...
```

Each check type gets its own RRD file (e.g., `ping.rrd`, `http.rrd`, `wifi_stations.rrd`). Multi-metric checks store all their data sources in a single RRD file.

## Makefile Targets

- **test**: Runs staticcheck and `go test` with race detection.
- **build**: Compiles the Go code and produces `wasgehtd`.
- **deps**: Verifies module dependencies and updates `go.mod` and `go.sum`.
- **clean**: Removes the `wasgehtd` binary and any generated graphs.
- **mrproper**: Removes all data including RRD files and generated graphs.

## Contributing

1. Fork the repository.
2. Create a new feature or bugfix branch.
3. Send a pull request (PR).

## License

MIT License

Copyright (c) 2026 Kyle Risse

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
