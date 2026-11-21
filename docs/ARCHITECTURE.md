# needlescope Architecture

## 1. Design Overview

`needlescope` is a Go engine designed for evaluating and mitigating attention degradation in large-context language models. The system profiles retrieval fidelity across context scales from 8k to 1M tokens, measures attention blind spots, and provides an inline proxy to re-order context chunks into optimal attention positions.

The binary executes with an RSS memory footprint under 20MB. It requires no external runtime dependencies, node packages, or python interpreters.

```
+-----------------------------------------------------------------+
|                       needlescope CLI                           |
|       benchmark | proxy | cockpit | evaluate | version          |
+-----------------------------------------------------------------+
                                |
        +-----------------------+-----------------------+
        |                                               |
        v                                               v
+-------------------------------+               +-------------------------------+
|     Benchmark Orchestrator    |               |  Salience Re-Arranger Proxy   |
|                               |               |                               |
|  - Fast BPE Tokenizer         |               |  - Boundary Chunker           |
|  - Synthetic Corpus Generator |               |  - BM25 Relevance Scorer      |
|  - Multi-Hop Needle Injector  |               |  - U-Shaped Attention Sched   |
|  - Multi-Provider LLM Client  |               |  - OpenAI-compatible Proxy    |
|  - 2D Length x Depth Sweep    |               |  - Latency Overhead < 5ms     |
+-------------------------------+               +-------------------------------+
        |                                               |
        v                                               v
+-------------------------------+               +-------------------------------+
| Attention Degradation Profile |               |     Embedded Web Cockpit      |
|                               |               |                               |
|  - 2D Recall Matrix           |               |  - HTTP Server on Port 8585   |
|  - Parabolic U-Curve Fit      |               |  - SSE Live Event Stream      |
|  - Blind Spot Window Bounds   |               |  - 2D Interactive Heatmap     |
|  - JSON and CSV Exporters     |               |  - Live Playground & Diff     |
+-------------------------------+               +-------------------------------+
```

---

## 2. Core Subsystems

### 2.1 Fast Tokenizer (`pkg/benchmark/tokenizer.go`)
Exact token depth calculation requires precise token placement. A needle targeted at relative depth $d = 0.45$ in a 64k context must sit at token offset $\lfloor 0.45 \times 64000 \rfloor$.

The tokenizer implements subword estimation matching BPE rules (cl100k_base and LLaMA conventions). It tracks word boundaries, whitespace clusters, and punctuation tokens with sublinear allocations. This allows exact depth positioning and fast text truncation without loading large external vocabulary binaries.

### 2.2 Multi-Provider Inference Client (`pkg/benchmark/client.go`)
The client interface abstracts model invocation across different backends:
- **Mock Client**: High-fidelity local simulation using empirical attention attenuation equations. It supports deterministic testing of single-needle, multi-needle, and multi-hop scenarios with zero API expense.
- **OpenAI Client**: Standard HTTP client compatible with OpenAI, vLLM, Ollama, LM Studio, and Groq endpoints.
- **Anthropic Client**: Implements the Anthropic Messages API format.

### 2.3 Needle and Corpus Generators (`pkg/benchmark/needles.go`, `corpus.go`)
- **Corpus Generation**: Produces synthetic domain haystacks (financial disclosures, legal contracts, technical cloud documentation, and clinical medical records) on demand. Contexts stream sequentially into buffers, maintaining low memory usage even at 256k tokens.
- **Single Needles**: Injects a unique key-value fact at depth $d \in [0.0, 1.0]$ with optional adversarial distractors placed at opposite depths.
- **Multi-Needles**: Injects $K$ distinct facts across multiple depths to evaluate cross-document aggregation.
- **Multi-Hop Dependency Chains**: Injects dependent facts at separate depths (such as Entity A -> Entity B at depth $d_1$, Entity B -> Secret Key at depth $d_2$). The query requires traversing the full chain.

### 2.4 Degradation Profiler (`pkg/profiler/`)
- **2D Recall Matrix**: Aggregates trial results by context length and token depth. Computes cell recall, average score, mean latency, and p95 latency.
- **U-Curve Regression**: Fits a second-order polynomial:
  $$R(d) = \alpha (d - 0.5)^2 + \gamma$$
  A positive $\alpha$ coefficient indicates performance degradation in the center.
- **Blind Spot Detector**: Identifies the contiguous depth interval $[d_{\min}, d_{\max}]$ where empirical recall falls below the target threshold (default 80%).

### 2.5 Salience Re-Arranger Reverse Proxy (`pkg/rearranger/`)
The proxy intercepts `/v1/chat/completions` requests:
1. Extracts context documents and the terminal user query.
2. Segments the context into semantic chunks using the **Boundary Shield**, which guarantees Markdown tables (`| ... |`), fenced code blocks (` ```...``` `), and XML elements remain unfragmented within atomic chunks.
3. Ranks chunks by query relevance using an in-memory BM25 scorer with phrase boosting.
4. Distributes chunks according to the mathematical U-distribution:
   - Rank 1 (highest salience) is placed in the recency position (end of context, before the query).
   - Rank 2 is placed in the primacy position (start of context, after system instructions).
   - Subsequent chunks alternate toward the center, leaving low-salience background text in the attention valley.
5. Proxies the reorganized request to the upstream LLM and streams the response back.
6. When `--shadow` mode is active, dispatches asynchronous dual-inference queries of the original prompt to compute live recall and latency lift in production without user disruption.

### 2.6 Quality Gate Engine (`pkg/profiler/gate.go`)
Automates SLA assertions for CI/CD pipelines. Evaluates candidate model degradation profiles against specified limits:
- Minimum middle-valley recall threshold ($0.35 \le d \le 0.65$).
- Maximum allowed edge-to-valley degradation delta.
- Maximum contiguous blind-spot width.
Returns exit code 0 on PASS, or exit code 1 to halt pipeline deployments when attention regressions occur.

### 2.7 Embedded Cockpit (`pkg/cockpit/`)
The cockpit serves on port 8585 using Go standard library embedded files:
- Interactive HTML5 canvas rendering the 2D degradation heatmap.
- Vector SVG charts showing empirical U-curves and fitted regression lines.
- Attention Gradient Spectrum Simulator visually projecting chunk positions onto the model attention window.
- Coordinate inspector displaying trial samples, model responses, and latency metrics.
- Live Re-Arranger playground for testing custom context inputs, showing chunk rankings and layout diffs.
- Server-Sent Events (SSE) stream for real-time progress updates during benchmark sweeps.

---

## 3. Concurrency and Memory Model

The benchmark runner uses a worker pool pattern. Concurrency is bounded by the `--concurrency` flag (default 4). Each worker handles one coordinate trial at a time.

Memory allocations are kept minimal:
- Pre-allocated string builders for corpus generation.
- In-memory BM25 index built on term frequency maps without persistent disk storage.
- Total memory usage remains below 20MB RSS during 128k to 256k token evaluations.
