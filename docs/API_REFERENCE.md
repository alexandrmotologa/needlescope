# needlescope API and CLI Reference

## 1. CLI Commands

### 1.1 `needlescope benchmark`
Executes a 2D grid needle retrieval stress test across context lengths and token depths.

```bash
needlescope benchmark [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|---|---|---|---|
| `--provider` | string | `mock` | Inference backend: `mock`, `openai`, `anthropic` |
| `--model` | string | `meta-llama/Llama-3.1-70B-Instruct` | Model name or upstream identifier |
| `--endpoint` | string | `http://localhost:8000/v1` | Upstream API URL for openai provider |
| `--api-key` | string | `$OPENAI_API_KEY` | Provider authentication key |
| `--min-tokens` | int | `8192` | Minimum context length in tokens |
| `--max-tokens` | int | `131072` | Maximum context length in tokens |
| `--step-tokens` | int | `16384` | Context length increment step |
| `--depth-steps` | int | `10` | Number of depth intervals between 0.0 and 1.0 |
| `--trials` | int | `2` | Number of evaluations per coordinate |
| `--type` | string | `single` | Needle type: `single`, `multi`, `multi-hop` |
| `--domain` | string | `finance` | Background corpus: `finance`, `legal`, `technical`, `healthcare` |
| `--concurrency` | int | `4` | Worker pool routine count |
| `--output` | string | `""` | Destination path for JSON report |
| `--csv` | string | `""` | Destination path for CSV matrix |
| `--cockpit` | bool | `false` | Launch embedded web cockpit upon completion |
| `--port` | int | `8585` | Cockpit port when `--cockpit` is set |

---

### 1.2 `needlescope proxy`
Runs the inline reverse proxy that intercepts `/v1/chat/completions` and reorganizes context chunks into the U-shaped attention layout.

```bash
needlescope proxy [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|---|---|---|---|
| `--listen` | string | `:8500` | Local listen address |
| `--upstream` | string | `https://api.openai.com/v1` | Target upstream LLM endpoint |
| `--strategy` | string | `u-shaped` | Reordering strategy: `u-shaped`, `recency-biased`, `original` |
| `--chunk-size` | int | `350` | Target tokens per document chunk |
| `--api-key` | string | `$OPENAI_API_KEY` | Default authorization key sent upstream |

---

### 1.3 `needlescope cockpit`
Launches the standalone web dashboard and interactive 2D heatmap on port 8585.

```bash
needlescope cockpit [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|---|---|---|---|
| `--port` | int | `8585` | HTTP listen port |
| `--host` | string | `0.0.0.0` | Bind host interface |
| `--report` | string | `""` | Path to existing JSON matrix report to display |

---

### 1.4 `needlescope evaluate`
Evaluates a specific long prompt file, calculates chunk salience, and outputs the reordered text.

```bash
needlescope evaluate [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|---|---|---|---|
| `--prompt-file` | string | *(required)* | Path to prompt text file |
| `--query` | string | `""` | Query to evaluate against |
| `--strategy` | string | `u-shaped` | Strategy: `u-shaped`, `recency-biased`, `original` |
| `--output` | string | `""` | Optional destination path for reordered prompt |

---

## 2. Reverse Proxy HTTP Endpoints (Port 8500)

### `POST /v1/chat/completions`
Standard OpenAI chat completions route. Reorganizes incoming prompt context documents into the U-shaped attention distribution before forwarding upstream.

**Injected Response Headers:**
- `X-Needlescope-Reordered`: `true`
- `X-Needlescope-Strategy`: `u-shaped`
- `X-Needlescope-Reorder-Time-Us`: Duration in microseconds
- `X-Needlescope-Chunks`: Number of chunks processed
- `X-Needlescope-Evicted-From-Middle`: Number of high-salience chunks moved out of the attention valley

### `GET /needlescope/stats`
Returns operational telemetry for the proxy:
```json
{
  "total_requests": 1420,
  "reordered_requests": 1184,
  "total_chunks_moved": 8920,
  "evicted_from_middle": 2341,
  "avg_reorder_time_us": 1280,
  "uptime_seconds": 3600
}
```

---

## 3. Cockpit HTTP API Endpoints (Port 8585)

### `GET /api/matrix`
Returns the active 2D attention degradation matrix, global U-curve regression coefficients, and critical blind-spot intervals.

### `POST /api/benchmark/start`
Starts a background benchmark sweep using the provided configuration payload.

### `POST /api/benchmark/stop`
Cancels the running sweep.

### `GET /api/benchmark/stream`
Server-Sent Events (SSE) connection streaming real-time trial progress. Emits events of type:
- `start`
- `trial_complete`
- `finished`
- `error`

### `POST /api/rearrange`
Calculates chunk salience and returns original vs reordered chunks for arbitrary input text:
```json
{
  "text": "...",
  "query": "...",
  "strategy": "u-shaped",
  "target_chunk_tokens": 200
}
```

### `GET /api/export/json`
Downloads the active matrix profile as formatted JSON.

### `GET /api/export/csv`
Downloads the active matrix profile as CSV.
