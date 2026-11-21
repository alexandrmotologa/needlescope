package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/alexandrmotologa/needlescope/pkg/benchmark"
	"github.com/alexandrmotologa/needlescope/pkg/cockpit"
	"github.com/alexandrmotologa/needlescope/pkg/profiler"
	"github.com/alexandrmotologa/needlescope/pkg/rearranger"
	"github.com/alexandrmotologa/needlescope/pkg/report"
)

const (
	version = "1.0.0"
	banner  = `
 ███╗   ██╗███████╗███████╗██████╗ ██╗     ███████╗███████╗ ██████╗ ██████╗ ██████╗ ███████╗
 ████╗  ██║██╔════╝██╔════╝██╔══██╗██║     ██╔════╝██╔════╝██╔════╝██╔═══██╗██╔══██╗██╔════╝
 ██╔██╗ ██║█████╗  █████╗  ██║  ██║██║     █████╗  ███████╗██║     ██║   ██║██████╔╝█████╗  
 ██║╚██╗██║██╔══╝  ██╔══╝  ██║  ██║██║     ██╔══╝  ╚════██║██║     ██║   ██║██╔═══╝ ██╔══╝  
 ██║ ╚████║███████╗███████╗██████╔╝███████╗███████╗███████║╚██████╗╚██████╔╝██║     ███████╗
 ╚═╝  ╚═══╝╚══════╝╚══════╝╚═════╝ ╚══════╝╚══════╝╚══════╝ ╚═════╝ ╚═════╝ ╚═╝     ╚══════╝
 Enterprise Long-Context Attention Degradation Profiler & Semantic Salience Re-Arranger
`
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "benchmark":
		runBenchmarkCmd(os.Args[2:])
	case "proxy":
		runProxyCmd(os.Args[2:])
	case "cockpit":
		runCockpitCmd(os.Args[2:])
	case "evaluate":
		runEvaluateCmd(os.Args[2:])
	case "gate":
		runGateCmd(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("needlescope version %s (Go runtime)\n", version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(banner)
	fmt.Println("Usage: needlescope <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  benchmark   Run a 2D grid needle retrieval evaluation across context lengths and depths")
	fmt.Println("  proxy       Start the dynamic semantic salience re-arranger reverse proxy (OpenAI compatible)")
	fmt.Println("  cockpit     Launch the embedded web dashboard and interactive 2D heatmap on port 8585")
	fmt.Println("  evaluate    Evaluate and re-order a specific enterprise prompt file")
	fmt.Println("  gate        Assert CI/CD quality gates on attention degradation reports")
	fmt.Println("  version     Print version and build details")
	fmt.Println("\nRun 'needlescope <command> --help' for details on specific options.")
}

func runBenchmarkCmd(args []string) {
	fs := flag.NewFlagSet("benchmark", flag.ExitOnError)

	provider := fs.String("provider", "mock", "LLM inference provider: mock, openai, anthropic")
	model := fs.String("model", "meta-llama/Llama-3.1-70B-Instruct", "Model name or ID")
	endpoint := fs.String("endpoint", "http://localhost:8000/v1", "API endpoint for openai provider")
	apiKey := fs.String("api-key", os.Getenv("OPENAI_API_KEY"), "API key for upstream provider")
	minTokens := fs.Int("min-tokens", 8192, "Minimum context length in tokens")
	maxTokens := fs.Int("max-tokens", 131072, "Maximum context length in tokens")
	stepTokens := fs.Int("step-tokens", 16384, "Context length increment step")
	depthSteps := fs.Int("depth-steps", 10, "Number of depth intervals between 0.0 and 1.0")
	trials := fs.Int("trials", 2, "Number of trials per (length, depth) coordinate")
	evalType := fs.String("type", "single", "Needle evaluation type: single, multi, multi-hop")
	domain := fs.String("domain", "finance", "Corpus domain: finance, legal, technical, healthcare")
	concurrency := fs.Int("concurrency", 4, "Concurrent worker dispatch routines")
	outputPath := fs.String("output", "", "Optional path to export JSON report")
	csvPath := fs.String("csv", "", "Optional path to export CSV matrix")
	htmlPath := fs.String("html", "", "Optional path to export self-contained HTML executive audit report")
	presetName := fs.String("preset", "", "Predefined frontier model preset: gpt-4o, claude-3-5-sonnet, llama-3.1-70b, gemini-1-5-pro, qwen-2.5-72b")
	launchCockpit := fs.Bool("cockpit", false, "Launch embedded web cockpit upon completion")
	cockpitPort := fs.Int("port", 8585, "Cockpit port when --cockpit is enabled")

	_ = fs.Parse(args)

	// Compute context lengths
	var lengths []int
	var depths []float64

	if *presetName != "" {
		preset, err := benchmark.GetPreset(*presetName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Preset error: %v\n", err)
			os.Exit(1)
		}
		lengths = preset.ContextLengths
		for i := 0; i <= preset.DepthSteps; i++ {
			depths = append(depths, float64(i)/float64(preset.DepthSteps))
		}
		if *model == "meta-llama/Llama-3.1-70B-Instruct" && preset.DefaultModel != "" {
			*model = preset.DefaultModel
		}
		if *domain == "finance" && preset.DefaultDomain != "" {
			*domain = string(preset.DefaultDomain)
		}
		fmt.Printf("[needlescope] Loaded preset %s: %s\n", preset.Name, preset.Description)
	} else {
		for l := *minTokens; l <= *maxTokens; l += *stepTokens {
			lengths = append(lengths, l)
		}
		if len(lengths) == 0 {
			lengths = []int{*minTokens}
		}

		if *depthSteps <= 1 {
			depths = []float64{0.0, 0.5, 1.0}
		} else {
			for i := 0; i <= *depthSteps; i++ {
				depths = append(depths, float64(i)/float64(*depthSteps))
			}
		}
	}

	cfg := benchmark.BenchmarkConfig{
		Provider:            *provider,
		Model:               *model,
		Endpoint:            *endpoint,
		APIKey:              *apiKey,
		ContextLengths:      lengths,
		Depths:              depths,
		TrialsPerCoordinate: *trials,
		Type:                benchmark.NeedleType(*evalType),
		Domain:              benchmark.CorpusDomain(*domain),
		Concurrency:         *concurrency,
	}

	var client benchmark.LLMClient
	switch *provider {
	case "openai":
		client = benchmark.NewOpenAIClient(*endpoint, *apiKey, *model)
	case "anthropic":
		client = benchmark.NewAnthropicClient(*apiKey, *model)
	default:
		client = benchmark.NewMockClient(benchmark.MockClientConfig{
			ModelName:      *model,
			MaxAttenuation: 0.58,
			Sigma:          0.20,
		})
	}

	runner := benchmark.NewRunner(cfg, client)
	events := runner.Subscribe()

	// Handle graceful termination
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down benchmark...")
		runner.Stop()
		cancel()
	}()

	totalTrials := len(lengths) * len(depths) * *trials
	fmt.Printf("[needlescope] Starting 2D Benchmark: %s (%s)\n", *model, *provider)
	fmt.Printf("Contexts: %d .. %d tokens (%d levels) | Depths: %d intervals | Total Trials: %d\n\n",
		lengths[0], lengths[len(lengths)-1], len(lengths), len(depths), totalTrials)

	// Terminal progress monitor
	go func() {
		for ev := range events {
			if ev.Type == "trial_complete" {
				pct := float64(ev.Completed) / float64(ev.TotalTrials) * 100
				status := "[PASS]"
				if !ev.LatestTrial.Success {
					status = "[FAIL]"
				}
				fmt.Printf("\r\033[K[%5.1f%%] %s Coord %s (%dt @ %.0f%%) - %dms",
					pct, status, ev.LatestTrial.CoordinateID, ev.CurrentLength, ev.CurrentDepth*100, ev.LatestTrial.LatencyMs)
			}
		}
	}()

	results, err := runner.Run(ctx)
	if err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "\nBenchmark failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n\nSweep complete. Aggregating degradation profile...")
	matrixReport := profiler.Aggregate(results, *model, *provider)

	// Print terminal colored report
	report.PrintTerminal(matrixReport, os.Stdout)

	// Export JSON
	if *outputPath != "" {
		_ = os.MkdirAll(filepath.Dir(*outputPath), 0755)
		if err := report.SaveJSON(matrixReport, *outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save JSON report: %v\n", err)
		} else {
			fmt.Printf("JSON report exported to: %s\n", *outputPath)
		}
	}

	// Export CSV
	if *csvPath != "" {
		_ = os.MkdirAll(filepath.Dir(*csvPath), 0755)
		if err := report.SaveCSV(matrixReport, *csvPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save CSV matrix: %v\n", err)
		} else {
			fmt.Printf("CSV matrix exported to: %s\n", *csvPath)
		}
	}

	// Export HTML
	if *htmlPath != "" {
		_ = os.MkdirAll(filepath.Dir(*htmlPath), 0755)
		if err := report.SaveHTML(matrixReport, *htmlPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save HTML report: %v\n", err)
		} else {
			fmt.Printf("HTML executive audit report exported to: %s\n", *htmlPath)
		}
	}

	// Launch cockpit if requested
	if *launchCockpit {
		cockpitServer := cockpit.NewServer(cockpit.ServerConfig{
			Port: *cockpitPort,
		}, matrixReport)
		if err := cockpitServer.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Cockpit error: %v\n", err)
		}
	}
}

func runProxyCmd(args []string) {
	fs := flag.NewFlagSet("proxy", flag.ExitOnError)

	listen := fs.String("listen", ":8500", "Listen address for reverse proxy")
	upstream := fs.String("upstream", "https://api.openai.com/v1", "Upstream target LLM API URL")
	strategy := fs.String("strategy", "u-shaped", "Attention re-ordering strategy: u-shaped, recency-biased, original")
	chunkSize := fs.Int("chunk-size", 350, "Target tokens per document chunk")
	apiKey := fs.String("api-key", os.Getenv("OPENAI_API_KEY"), "Default API key forwarded to upstream")
	shadowMode := fs.Bool("shadow", false, "Enable asynchronous dual-inference shadow testing to compare original vs reordered recall")

	_ = fs.Parse(args)

	cfg := rearranger.ProxyConfig{
		ListenAddr:        *listen,
		UpstreamURL:       *upstream,
		Strategy:          rearranger.StrategyType(*strategy),
		TargetChunkTokens: *chunkSize,
		DefaultAPIKey:     *apiKey,
		ShadowMode:        *shadowMode,
	}

	proxy, err := rearranger.NewProxy(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize proxy: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(banner)
	fmt.Printf("[needlescope] Semantic Salience Re-Arranger Proxy active\n")
	fmt.Printf("  Listen:    %s\n", *listen)
	fmt.Printf("  Upstream:  %s\n", *upstream)
	fmt.Printf("  Strategy:  %s (BM25 + Salience U-Shape)\n\n", *strategy)

	if err := proxy.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Proxy crashed: %v\n", err)
		os.Exit(1)
	}
}

func runCockpitCmd(args []string) {
	fs := flag.NewFlagSet("cockpit", flag.ExitOnError)

	port := fs.Int("port", 8585, "HTTP listen port")
	host := fs.String("host", "0.0.0.0", "HTTP host interface")
	reportFile := fs.String("report", "", "Path to pre-existing JSON matrix report")

	_ = fs.Parse(args)

	var loadedReport *profiler.MatrixReport
	if *reportFile != "" {
		data, err := os.ReadFile(*reportFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read report file %s: %v\n", *reportFile, err)
		} else {
			var rep profiler.MatrixReport
			if err := json.Unmarshal(data, &rep); err == nil {
				loadedReport = &rep
				fmt.Printf("[needlescope] Loaded report from %s (%d trials)\n", *reportFile, loadedReport.TotalTrials)
			}
		}
	}

	server := cockpit.NewServer(cockpit.ServerConfig{
		Port:       *port,
		Host:       *host,
		ReportPath: *reportFile,
	}, loadedReport)

	fmt.Print(banner)
	fmt.Printf("[needlescope] Web Cockpit running at http://localhost:%d\n\n", *port)

	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Cockpit server error: %v\n", err)
		os.Exit(1)
	}
}

func runEvaluateCmd(args []string) {
	fs := flag.NewFlagSet("evaluate", flag.ExitOnError)

	promptFile := fs.String("prompt-file", "", "Path to text file containing long prompt")
	query := fs.String("query", "", "Target user query")
	strategy := fs.String("strategy", "u-shaped", "Reordering strategy: u-shaped, recency-biased, original")
	output := fs.String("output", "", "Optional path to write reordered prompt")

	_ = fs.Parse(args)

	if *promptFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --prompt-file is required")
		fs.Usage()
		os.Exit(1)
	}

	contentBytes, err := os.ReadFile(*promptFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read prompt file: %v\n", err)
		os.Exit(1)
	}

	tok := benchmark.NewFastTokenizer()
	chunker := rearranger.NewChunker(tok, 300)
	scorer := rearranger.NewBM25Scorer()

	rawText := string(contentBytes)
	prefix, contextBody, extractedQuery := rearranger.ExtractPromptContext(rawText)
	if *query != "" {
		extractedQuery = *query
	}

	chunks := chunker.SplitChunks(contextBody)
	scoredChunks := scorer.ScoreChunks(chunks, extractedQuery)
	res := rearranger.Reorder(scoredChunks, rearranger.StrategyType(*strategy))

	fmt.Printf("[needlescope] Prompt Evaluation Summary:\n")
	fmt.Printf("  Total Tokens:              %d\n", res.TotalTokens)
	fmt.Printf("  Extracted Chunks:          %d\n", res.ChunkCount)
	fmt.Printf("  Middle Valley Evictions:   %d\n", res.MiddleValleyEvictionCount)
	fmt.Printf("  Estimated Baseline Recall: %.1f%%\n", res.EstimatedBaselineRecall*100)
	fmt.Printf("  Estimated U-Shape Recall:  %.1f%% (+%.1f%% gain)\n\n",
		res.EstimatedReorderedRecall*100, res.ExpectedGainPercent)

	finalPrompt := res.ReorderedContext
	if prefix != "" {
		finalPrompt = prefix + finalPrompt
	}
	if extractedQuery != "" {
		finalPrompt = finalPrompt + "\n\nQUERY:\n" + extractedQuery
	}

	if *output != "" {
		_ = os.WriteFile(*output, []byte(finalPrompt), 0644)
		fmt.Printf("Re-ordered prompt written to: %s\n", *output)
	}
}

func runGateCmd(args []string) {
	fs := flag.NewFlagSet("gate", flag.ExitOnError)

	reportFile := fs.String("report", "", "Path to JSON matrix report (required)")
	minMiddle := fs.Float64("min-middle-recall", 0.80, "Minimum required recall in middle valley (0.35 - 0.65)")
	maxDelta := fs.Float64("max-degradation-delta", 0.20, "Maximum allowed degradation delta between edges and valley")
	minOverall := fs.Float64("min-overall-recall", 0.85, "Minimum required overall recall across all coordinates")
	maxBlindSpot := fs.Float64("max-blindspot-width", 0.15, "Maximum allowed blind spot depth span")
	jsonOutput := fs.Bool("json", false, "Output assertions as JSON")

	_ = fs.Parse(args)

	if *reportFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --report is required for gate evaluation")
		fs.Usage()
		os.Exit(1)
	}

	data, err := os.ReadFile(*reportFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read report file %s: %v\n", *reportFile, err)
		os.Exit(1)
	}

	var rep profiler.MatrixReport
	if err := json.Unmarshal(data, &rep); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid JSON in report file: %v\n", err)
		os.Exit(1)
	}

	gateCfg := profiler.GateConfig{
		MinMiddleRecall:     *minMiddle,
		MaxDegradationDelta: *maxDelta,
		MinOverallRecall:    *minOverall,
		MaxBlindSpotWidth:   *maxBlindSpot,
	}

	result := profiler.EvaluateGate(&rep, gateCfg)

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		if !result.Passed {
			os.Exit(1)
		}
		return
	}

	fmt.Printf("\n=== needlescope Attention Quality Gate ===\n")
	fmt.Printf("Model: %s | Evaluated Rules: %d\n\n", rep.Model, len(result.Assertions))

	for _, a := range result.Assertions {
		tag := "\033[32m[PASS]\033[0m"
		if !a.Passed {
			tag = "\033[31m[FAIL]\033[0m"
		}
		fmt.Printf("  %s %-32s : %s\n", tag, a.Rule, a.Message)
	}

	fmt.Println()
	if result.Passed {
		fmt.Printf("\033[32m✔ QUALITY GATE PASSED:\033[0m Model meets all attention retention SLA requirements.\n\n")
		os.Exit(0)
	} else {
		fmt.Fprintf(os.Stderr, "\033[31m✘ QUALITY GATE FAILED:\033[0m Model exhibits unacceptable attention degradation.\n\n")
		os.Exit(1)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func parsePort(s string) int {
	p, err := strconv.Atoi(strings.TrimPrefix(s, ":"))
	if err != nil || p <= 0 {
		return 8585
	}
	return p
}
