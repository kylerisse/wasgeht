# Was Geht

## Overview

**Was Geht** is a small Go application that pings a list of hosts at regular intervals, tracks their availability (UP or DOWN), and records the latency in a Round Robin Database (RRD). A lightweight web interface serves host status information and interactive graphs of the recorded latency.

## Features

- **Ping Monitoring**: Sends ICMP Echo Requests to check host availability.
- **Latency Logging**: Uses RRD to store latency data over time.
- **Graphs Generation**: Generates historical latency graphs (15 minutes, 4 hours, 8 hours, etc.) for each host.
- **Simple Web Interface**: Serves an HTML/JS front-end to display host status and dynamically loaded graphs.
- **REST API**: Exposes JSON data of all hosts and their status at `GET /api`.

## Requirements


### Using Nix (Recommended)

If you have **Nix** installed, you can simply enter a development shell with all required dependencies using:

```bash
nix-shell
```

This loads the environment specified in `shell.nix`:

- Go (for building),
- gnumake (for Makefile),
- rrdtool (for handling RRD databases),
- unixtools.ping (ping utility).

Once inside the shell, you can run the usual make commands

### Without Nix ###

Ensure the following are installed:

- **Go** (1.23+ recommended)
- **rrdtool** and **iputils ping** must be installed and available on the system path.
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

## Makefile Targets

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

Copyright (c) 2025 Kyle Risse

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
