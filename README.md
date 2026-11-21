# needlescope

Long-Context Attention Degradation Profiler and Semantic Salience Re-Arranger in Go.

`needlescope` profiles attention degradation across 128k to 1M token context windows, maps empirical retrieval blind spots on a 2D grid (Context Length vs Token Depth), and provides a low-latency proxy that eliminates the "Lost in the Middle" phenomenon through mathematical U-shaped salience re-ordering.

---

## Technical Highlights

- **Low Memory Footprint**: Runs with an RSS memory footprint under 20MB (0.28MB heap during 128k token processing).
- **2D Degradation Matrix**: Sweeps across context lengths (8k to 1M tokens) and insertion depths (0% to 100%) to measure cell-by-cell recall, latency percentiles, and variance.
- **Parametric U-Curve Regression**: Fits empirical attention recall to second-order parabolic profiles $R(d) = \alpha (d - 0.5)^2 + \gamma$, quantifying the curvature coefficient $\alpha$ and identifying the critical blind-spot window $[d_{\min}, d_{\max}]$.
- **Multi-Hop Dependency Reasoning**: Evaluates multi-needle aggregation and chained directional dependencies with adversarial distractors.
- **Dynamic Salience Re-Arranger**: An inline OpenAI-compatible reverse proxy that parses document chunks, calculates BM25 query salience, and maps relevant text into optimal primacy and recency zones, raising middle-depth recall from ~50% to over 95%.
- **Zero-Dependency Web Cockpit**: Embedded HTTP dashboard on port 8585 with interactive 2D heatmaps, vector U-curve charts, and real-time Server-Sent Events (SSE).

---

## Installation

### From Source (Go 1.22+)

```bash
git clone https://github.com/alexandrmotologa/needlescope.git
cd needlescope
go build -o needlescope.exe ./cmd/needlescope
```

Verify the installation:

```bash
./needlescope.exe version
```

---

## Quick Start

### 1. Run a 2D Attention Benchmark

Run a multi-coordinate sweep using the built-in deterministic mock provider (requires no external API keys):

```bash
needlescope benchmark \
  --provider mock \
  --min-tokens 8192 \
  --max-tokens 65536 \
  --step-tokens 16384 \
  --depth-steps 10 \
  --trials 2 \
  --type single \
  --domain finance \
  --output ./reports/eval_matrix.json \
  --csv ./reports/eval_matrix.csv
```

To run against a live OpenAI, vLLM, or Ollama endpoint:

```bash
needlescope benchmark \
  --provider openai \
  --endpoint "http://localhost:8000/v1" \
  --model "meta-llama/Llama-3.1-70B-Instruct" \
  --min-tokens 8192 \
  --max-tokens 131072 \
  --cockpit
```

### 2. Start the Salience Re-Arranger Reverse Proxy

Run the inline proxy on port 8500 in front of any OpenAI-compatible server:

```bash
needlescope proxy \
  --listen ":8500" \
  --upstream "https://api.openai.com/v1" \
  --strategy u-shaped \
  --chunk-size 350
```

Redirect your application requests to `http://localhost:8500/v1/chat/completions`. Incoming context blocks are automatically tokenized, scored via BM25 against the user query, and reorganized into a U-shaped attention layout before forwarding upstream.

The proxy injects inspection headers into every response:
- `X-Needlescope-Reordered`: `true`
- `X-Needlescope-Strategy`: `u-shaped`
- `X-Needlescope-Reorder-Time-Us`: Latency overhead in microseconds (typically < 3000 µs)
- `X-Needlescope-Evicted-From-Middle`: Count of high-salience chunks moved out of the central valley

### 3. Launch the Interactive Web Cockpit

Start the cockpit on port 8585:

```bash
needlescope cockpit --port 8585
```

Open `http://localhost:8585` in your browser to inspect 2D heatmaps, view U-curve regression parabolas, run live benchmarks, and test the real-time prompt re-arranger playground.

### 4. Evaluate an Existing Prompt File

Analyze an existing long-context enterprise prompt and output the optimized text:

```bash
needlescope evaluate \
  --prompt-file ./docs/examples/enterprise_sec_filing.txt \
  --query "What is the primary administrative clearance token?" \
  --strategy u-shaped \
  --output ./reordered_prompt.txt
```

---

## Architectural Workflow

```
Incoming Request (Client)
        |
        v
+--------------------------------------------------------------+
|             needlescope Salience Re-Arranger                 |
|                                                              |
|  1. Extract prompt instructions, context body, and query    |
|  2. Segment context by markdown headers or paragraph breaks  |
|  3. Compute Okapi BM25 scores against user query terms       |
|  4. Map chunks into U-distribution:                          |
|     - Rank 1 (Most Salient)  -> Recency (Last slot)          |
|     - Rank 2                 -> Primacy (First slot)         |
|     - Lowest Salience        -> Central Attention Valley     |
+--------------------------------------------------------------+
        |
        v  (Reorganized Prompt, Latency Overhead < 5ms)
+--------------------------------------------------------------+
|              Upstream Large Language Model                   |
|       (OpenAI / vLLM / Ollama / Anthropic / Gemini)          |
+--------------------------------------------------------------+
        |
        v
Response Streamed Back to Client with Telemetry Headers
```

---

## Performance and Recall Impact

Empirical testing across 11 depth levels on long-context benchmarks demonstrates the following recall differences:

| Context Scale | Metric | Baseline (Raw) | needlescope (U-Shaped) | Net Delta |
|---|---|---|---|---|
| **32k tokens** | Attention Valley ($0.35 \le d \le 0.65$) | 68.0% | 96.0% | **+28.0%** |
| | Overall Average | 84.2% | 97.2% | +13.0% |
| **64k tokens** | Attention Valley ($0.35 \le d \le 0.65$) | 54.0% | 94.0% | **+40.0%** |
| | Overall Average | 77.8% | 95.8% | +18.0% |
| **128k tokens** | Attention Valley ($0.35 \le d \le 0.65$) | 42.0% | 94.0% | **+52.0%** |
| | Overall Average | 71.2% | 95.4% | +24.2% |

For detailed mathematical proofs, benchmarks, and multi-hop dependency evaluations, see:
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- [docs/MATHEMATICS.md](docs/MATHEMATICS.md)
- [docs/API_REFERENCE.md](docs/API_REFERENCE.md)
- [docs/REARRANGER_BENCHMARK.md](docs/REARRANGER_BENCHMARK.md)

---

## Testing

Run all unit, integration, and memory benchmark tests:

```bash
go test -v ./...
```

Run only memory footprint verification:

```bash
go test -v -run TestMemoryFootprintUnder20MB ./test
```

---

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
