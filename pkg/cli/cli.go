package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"svcprobe/pkg/graph"
	"svcprobe/pkg/probe"
	"svcprobe/pkg/reporter"
	"svcprobe/pkg/watch"
)

const Version = "1.0.0"

// Run parses arguments and executes the requested command. Returns process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stdout)
		return 0
	}

	command := args[1]
	cmdArgs := args[2:]

	// Setup context with interrupt handling (Ctrl+C / SIGTERM)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	switch command {
	case "check":
		return runProbeCommand(ctx, probe.ProbeCheck, cmdArgs, stdout, stderr)
	case "dns":
		return runProbeCommand(ctx, probe.ProbeDNS, cmdArgs, stdout, stderr)
	case "tcp":
		return runProbeCommand(ctx, probe.ProbeTCP, cmdArgs, stdout, stderr)
	case "tls":
		return runProbeCommand(ctx, probe.ProbeTLS, cmdArgs, stdout, stderr)
	case "http":
		return runProbeCommand(ctx, probe.ProbeHTTP, cmdArgs, stdout, stderr)
	case "watch":
		return runWatchCommand(ctx, cmdArgs, stdout, stderr)
	case "graph":
		return runGraphCommand(ctx, cmdArgs, stdout, stderr)
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "svcprobe v%s (network diagnostic CLI for distributed services)\n", Version)
		return 0
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		// If command starts with '-' or looks like an endpoint, assume 'check' command
		if strings.HasPrefix(command, "-") || strings.Contains(command, ".") || strings.Contains(command, ":") || command == "localhost" {
			return runProbeCommand(ctx, probe.ProbeCheck, args[1:], stdout, stderr)
		}
		fmt.Fprintf(stderr, "Unknown command '%s'. Run 'svcprobe --help' for usage.\n", command)
		return 1
	}
}

func runProbeCommand(ctx context.Context, forcedType probe.ProbeType, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		samples       int
		timeout       time.Duration
		interval      time.Duration
		concurrency   int
		outputFormat  string
		dnsServer     string
		slowThreshold time.Duration
		httpMethod    string
		insecure      bool
		noColor       bool
	)

	fs.IntVar(&samples, "n", 5, "Number of probe samples per endpoint")
	fs.IntVar(&samples, "samples", 5, "Number of probe samples per endpoint")
	fs.DurationVar(&timeout, "t", 5*time.Second, "Timeout per probe sample")
	fs.DurationVar(&timeout, "timeout", 5*time.Second, "Timeout per probe sample")
	fs.DurationVar(&interval, "i", 200*time.Millisecond, "Interval between samples")
	fs.DurationVar(&interval, "interval", 200*time.Millisecond, "Interval between samples")
	fs.IntVar(&concurrency, "c", 10, "Maximum concurrent probes")
	fs.IntVar(&concurrency, "concurrency", 10, "Maximum concurrent probes")
	fs.StringVar(&outputFormat, "o", "table", "Output format: table, json, json-pretty")
	fs.StringVar(&outputFormat, "output", "table", "Output format: table, json, json-pretty")
	fs.StringVar(&dnsServer, "server", "", "Custom DNS server resolver (e.g. 8.8.8.8:53)")
	fs.DurationVar(&slowThreshold, "slow-threshold", 500*time.Millisecond, "Latency threshold for slow service warning")
	fs.StringVar(&httpMethod, "method", "GET", "HTTP method (for http probe)")
	fs.BoolVar(&insecure, "k", false, "Skip TLS certificate verification")
	fs.BoolVar(&insecure, "insecure", false, "Skip TLS certificate verification")
	fs.BoolVar(&noColor, "no-color", false, "Disable ANSI colorized output")

	targets, err := parseFlagsAndTargets(fs, args)
	if err != nil {
		return 1
	}

	if len(targets) == 0 {
		fmt.Fprintln(stderr, "Error: no target endpoints provided. Example: svcprobe check https://api.example.com")
		return 1
	}

	opts := probe.DefaultOptions()
	opts.Samples = samples
	opts.Timeout = timeout
	opts.Interval = interval
	opts.Concurrency = concurrency
	opts.DNSServer = dnsServer
	opts.SlowThreshold = slowThreshold
	opts.HTTPMethod = httpMethod
	opts.InsecureSkipVerify = insecure

	suite, err := probe.RunSuite(ctx, targets, forcedType, opts)
	if err != nil {
		fmt.Fprintf(stderr, "Probe suite execution failed: %v\n", err)
		return 1
	}

	colorize := !noColor && isTerminalWriter(stdout)

	switch strings.ToLower(outputFormat) {
	case "json":
		_ = reporter.PrintJSON(stdout, suite, false)
	case "json-pretty":
		_ = reporter.PrintJSON(stdout, suite, true)
	default:
		reporter.PrintTable(stdout, suite, colorize)
	}

	if suite.UnhealthyCount > 0 {
		return 2 // Exit code 2 indicates unhealthy service(s)
	}
	return 0
}

func runWatchCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		interval     time.Duration
		timeout      time.Duration
		concurrency  int
		outputFormat string
		noColor      bool
	)

	fs.DurationVar(&interval, "i", 2*time.Second, "Watch interval between runs")
	fs.DurationVar(&interval, "interval", 2*time.Second, "Watch interval between runs")
	fs.DurationVar(&timeout, "t", 5*time.Second, "Timeout per probe sample")
	fs.DurationVar(&timeout, "timeout", 5*time.Second, "Timeout per probe sample")
	fs.IntVar(&concurrency, "c", 10, "Maximum concurrent probes")
	fs.IntVar(&concurrency, "concurrency", 10, "Maximum concurrent probes")
	fs.StringVar(&outputFormat, "o", "table", "Output format: table, json")
	fs.StringVar(&outputFormat, "output", "table", "Output format: table, json")
	fs.BoolVar(&noColor, "no-color", false, "Disable ANSI colorized output")

	targets, err := parseFlagsAndTargets(fs, args)
	if err != nil {
		return 1
	}

	if len(targets) == 0 {
		fmt.Fprintln(stderr, "Error: no target endpoints provided for watch mode.")
		return 1
	}

	opts := probe.DefaultOptions()
	opts.Samples = 1
	opts.Timeout = timeout
	opts.Interval = interval
	opts.Concurrency = concurrency

	colorize := !noColor && isTerminalWriter(stdout)

	w := watch.NewWatcher(targets, probe.ProbeCheck, opts, func(iteration int, suite probe.SuiteResult, statsMap map[string]*watch.TargetStats) {
		if strings.ToLower(outputFormat) == "json" {
			_ = reporter.PrintJSON(stdout, suite, false)
			return
		}

		if colorize {
			// Clear screen ansi code for interactive watch updates
			fmt.Fprint(stdout, "\033[H\033[2J")
		}
		fmt.Fprintf(stdout, "[WATCH MODE] Iteration #%d (Press Ctrl+C to stop)\n", iteration+1)
		reporter.PrintTable(stdout, suite, colorize)
	})

	fmt.Fprintf(stdout, "Starting continuous watch monitoring on %d endpoint(s) every %v...\n", len(targets), interval)
	err = w.Start(ctx)
	if err != nil && err != context.Canceled {
		fmt.Fprintf(stderr, "Watch terminated with error: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "\nWatch monitoring stopped.")
	return 0
}

func runGraphCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		samples     int
		timeout     time.Duration
		concurrency int
		noColor     bool
	)

	fs.IntVar(&samples, "n", 3, "Number of probe samples per endpoint")
	fs.IntVar(&samples, "samples", 3, "Number of probe samples per endpoint")
	fs.DurationVar(&timeout, "t", 5*time.Second, "Timeout per probe sample")
	fs.DurationVar(&timeout, "timeout", 5*time.Second, "Timeout per probe sample")
	fs.IntVar(&concurrency, "c", 10, "Maximum concurrent probes")
	fs.IntVar(&concurrency, "concurrency", 10, "Maximum concurrent probes")
	fs.BoolVar(&noColor, "no-color", false, "Disable ANSI colorized output")

	targets, err := parseFlagsAndTargets(fs, args)
	if err != nil {
		return 1
	}

	if len(targets) == 0 {
		fmt.Fprintln(stderr, "Error: no target endpoints provided for graph topology.")
		return 1
	}

	opts := probe.DefaultOptions()
	opts.Samples = samples
	opts.Timeout = timeout
	opts.Concurrency = concurrency

	suite, err := probe.RunSuite(ctx, targets, probe.ProbeCheck, opts)
	if err != nil {
		fmt.Fprintf(stderr, "Graph suite execution failed: %v\n", err)
		return 1
	}

	colorize := !noColor && isTerminalWriter(stdout)
	output := graph.RenderGraph(suite, colorize)
	fmt.Fprint(stdout, output)

	if suite.UnhealthyCount > 0 {
		return 2
	}
	return 0
}

func parseFlagsAndTargets(fs *flag.FlagSet, args []string) ([]string, error) {
	var flagArgs []string
	var targetArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if !strings.Contains(arg, "=") {
				name := strings.TrimLeft(arg, "-")
				if isValueFlag(fs, name) && i+1 < len(args) {
					i++
					flagArgs = append(flagArgs, args[i])
				}
			}
		} else {
			targetArgs = append(targetArgs, arg)
		}
	}

	reordered := append(flagArgs, targetArgs...)
	if err := fs.Parse(reordered); err != nil {
		return nil, err
	}
	return fs.Args(), nil
}

func isValueFlag(fs *flag.FlagSet, name string) bool {
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	type boolFlag interface {
		IsBoolFlag() bool
	}
	if b, ok := f.Value.(boolFlag); ok && b.IsBoolFlag() {
		return false
	}
	return true
}

func isTerminalWriter(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		fi, err := f.Stat()
		if err == nil {
			return (fi.Mode() & os.ModeCharDevice) != 0
		}
	}
	return false
}

func printUsage(w io.Writer) {
	usage := `svcprobe - Distributed Service & Network Diagnostic CLI (v` + Version + `)

Usage:
  svcprobe <command> [flags] <endpoints...>

Commands:
  check      Perform comprehensive multi-layer diagnostic check (DNS -> TCP -> TLS -> HTTP)
  watch      Continuously monitor endpoints with real-time statistics
  dns        Execute DNS resolution probe (measure lookup time, records, CNAME)
  tcp        Execute TCP handshake probe (measure socket connect time)
  tls        Execute TLS handshake and certificate validity inspection
  http       Execute HTTP probe with latency phase decomposition (TTFB, Transfer, DNS, TCP, TLS)
  graph      Render ASCII topology & service dependency latency visual
  version    Show version info

Flags:
  -n, --samples INT            Number of probe samples per endpoint (default: 5)
  -t, --timeout DURATION       Timeout per probe sample (default: 5s)
  -i, --interval DURATION      Interval between samples / watch loops (default: 200ms)
  -c, --concurrency INT        Max concurrent worker probes (default: 10)
  -o, --output STRING          Output format: 'table', 'json', 'json-pretty' (default: table)
      --server STRING          Custom DNS server resolver (e.g. 8.8.8.8:53)
      --slow-threshold DURATION Latency threshold for slow service warning (default: 500ms)
      --method STRING          HTTP method for HTTP probe (default: GET)
  -k, --insecure               Skip TLS certificate verification
      --no-color               Disable ANSI color output

Examples:
  # Comprehensive health check across multiple services
  svcprobe check https://api.github.com http://localhost:8080 tcp://db.internal:5432

  # Dedicated TLS certificate inspection
  svcprobe tls google.com:443 --samples 3

  # HTTP latency phase breakdown exported to JSON
  svcprobe http https://httpbin.org/delay/1 -o json-pretty

  # Continuous service health watcher
  svcprobe watch https://api.mycompany.com --interval 3s

  # Visual topology & latency percentiles
  svcprobe graph https://api.github.com https://google.com
`
	fmt.Fprint(w, usage)
}
