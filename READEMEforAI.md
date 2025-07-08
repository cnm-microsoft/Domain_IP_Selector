# Domain IP Selector (Go Edition) - AI Developer Documentation

## 1. Project Overview

This project is a high-performance Cloudflare IP address selection tool written in Go. It identifies the optimal Cloudflare IPs for a user's network environment by performing a series of tests, including DNS resolution, latency checks, and download speed measurements. The application is designed to be a single, dependency-free executable and supports two operational modes: a user-friendly web interface and a standard command-line interface (CLI).

## 2. System Architecture

The application is composed of several distinct components that interact to form a processing pipeline. The user initiates a task through either the Web UI or the CLI, which then triggers the core `Engine`.

```mermaid
graph TD
    subgraph User Interface
        A[Web UI] -- WebSocket --> C{Server}
        B[CLI] -- Flags --> D{CLI Handler}
    end

    subgraph Core Logic
        C -- Invokes --> E[Engine]
        D -- Invokes --> E[Engine]
    end

    subgraph Engine Pipeline
        E -- 1. Load --> F[Data Sources]
        E -- 2. Resolve & Filter --> G[DNS & CF IP Filter]
        E -- 3. Test Latency --> H[Latency Tester]
        E -- 4. Group --> I[Group & Sort]
        E -- 5. Test Speed --> J[Speed Tester]
    end

    subgraph Data & IO
        F -- reads -->|config.yaml| E
        F -- reads -->|locations.json| E
        F -- reads -->|reputation_domains.txt| E
        E -- writes --> K[Output Files .json/.csv]
    end

    style A fill:#cde4ff
    style B fill:#cdffd8
```

## 3. Component Deep Dive

*   **`cmd`**: The main entry point of the application. It handles command-line flag parsing (e.g., `--cli`) to determine the operational mode. It also embeds default configuration files (`default_config.yaml`, `locations.json`, `reputation_domains.txt`) which are created on the first run.
*   **`internal/config`**: Defines the `Config` struct that maps to the `config.yaml` file. It provides the `LoadConfig` function to read and unmarshal the YAML configuration.
*   **`internal/engine`**: This is the core orchestrator. The `Run` function executes the entire IP selection pipeline, from data loading to final result generation, invoking other components in sequence.
*   **`internal/datasource`**: Manages the loading of external data: the official Cloudflare IP ranges (`cf-ips-v4.txt`, `cf-ips-v6.txt`) and the list of domains to be resolved (`reputation_domains.txt`).
*   **`internal/tester`**: Implements the network testing logic. `TestLatency` uses an `httping`-like mechanism against `cloudflare.com/cdn-cgi/trace` to measure latency, packet loss, and retrieve the Colo ID. `TestDownloadSpeed` measures throughput from Cloudflare's speed test servers.
*   **`internal/locations`**: Provides the functionality to load `locations.json`, which maps Cloudflare Colo IDs (e.g., "SJC") to human-readable region names (e.g., "North America").
*   **`internal/output`**: Handles the serialization and writing of the final results into both JSON (`result_*.json`) and CSV (`result_*.csv`) formats.
*   **`internal/server`**: Implements the web server mode. It serves the embedded static frontend files (HTML/CSS/JS). Key API endpoints include:
    *   `/api/config`: A RESTful endpoint for GETting and POSTing configuration changes. It intelligently preserves comments in the YAML file when saving.
    *   `/api/locations`: Provides a structured list of available regions and colos for the frontend UI.
    *   `/ws/run`: The WebSocket endpoint that orchestrates the `engine.Run` process. It receives configuration from the client, streams real-time log messages back, and finally sends the complete result set.

## 4. Detailed Workflow

### Web Mode Workflow

1.  User runs `main.exe`.
2.  The `server` starts, serves the embedded `index.html`, and attempts to open `http://localhost:8080` in the user's browser.
3.  Frontend fetches initial data from `/api/config` and `/api/locations`.
4.  User modifies settings in the UI and clicks "Start".
5.  Frontend establishes a WebSocket connection to `/ws/run` and sends the current configuration as a JSON message.
6.  The server-side WebSocket handler receives the config, and invokes `engine.Run`, passing a callback function.
7.  The engine executes its pipeline. The callback function is called at each step, sending a `WebSocketMessage` of type `log` to the client.
8.  Upon completion, the engine returns the final results. The WebSocket handler sends a final `WebSocketMessage` of type `result` containing the array of `SimplifiedResult`.
9.  The results are also saved to `web_result_*.csv` and `web_result_*.json`.
10. The connection is closed.

### CLI Mode Workflow

1.  User runs `main.exe --cli`.
2.  The `main` function in `cmd/main.go` detects the `--cli` flag.
3.  `runCli` function is called.
4.  `config.LoadConfig` is called to load `config.yaml`.
5.  `engine.Run` is called directly with a callback that prints logs to `stdout`.
6.  The engine executes its full pipeline.
7.  The final results are written to `result_*.csv` and `result_*.json` by the `output` package.

## 5. Configuration (`config.yaml`) Reference

This file controls the behavior of the engine.

| Key                      | Type      | Description                                                                                             |
| ------------------------ | --------- | ------------------------------------------------------------------------------------------------------- |
| `dns_concurrency`        | `int`     | Number of concurrent DNS resolutions.                                                                   |
| `latency_test_concurrency` | `int`     | Number of concurrent latency tests.                                                                     |
| `speedtest_concurrency`  | `int`     | Number of concurrent download speed tests.                                                              |
| `max_latency`            | `int`     | Maximum acceptable latency in milliseconds. IPs exceeding this are discarded.                           |
| `top_n_per_group`        | `int`     | Number of top IPs (by speed) to select from each group (colo or region).                                |
| `ip_version`             | `string`  | IP version to test. Can be `"ipv4"` or `"ipv6"`.                                                          |
| `speedtest_rate_limit_mb`| `float64` | Limits the bandwidth usage for each speed test in Megabytes/sec to prevent network saturation.          |
| `group_by`               | `string`  | How to group IPs for the final speed test. Can be `"colo"` or `"region"`.                                 |
| `filter_regions`         | `[]string`| A list of regions to include. If not empty, only IPs from these regions will be tested. Example: `["North America"]`. |
| `filter_colos`           | `[]string`| A list of colos to include. If not empty, only IPs from these colos will be tested. Example: `["SJC", "LAX"]`. |
| `min_speed`              | `float64` | Minimum acceptable download speed in MB/s. IPs below this speed are discarded.                          |

## 6. Data Models

*   **`model.IPInfo`**:
    *   `Address net.IP`: The resolved IP address.
    *   `SourceDomain string`: The domain from which this IP was resolved.

*   **`model.LatencyResult`**:
    *   `IPInfo model.IPInfo`: The original IP info.
    *   `Delay time.Duration`: Measured latency.
    *   `LossRate float64`: Packet loss rate (0.0 to 1.0).
    *   `Colo string`: Cloudflare data center ID (e.g., "SJC").
    *   `Region string`: Human-readable region (e.g., "North America").

*   **`engine.SimplifiedResult`**: The final, flattened data structure for output.
    *   `Address string`: IP address.
    *   `SourceDomain string`: Original source domain.
    *   `Delay int64`: Latency in nanoseconds.
    *   `LossRate float64`: Packet loss rate.
    *   `Colo string`: Data center ID.
    *   `Region string`: Geographic region.
    *   `DownloadSpeed int`: Download speed in KB/s.