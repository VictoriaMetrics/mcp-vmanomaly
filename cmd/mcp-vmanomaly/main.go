package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/VictoriaMetrics/mcp-vmanomaly/cmd/mcp-vmanomaly/config"
	"github.com/VictoriaMetrics/mcp-vmanomaly/cmd/mcp-vmanomaly/hooks"
	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/promts"
	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/resources"
	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/tools"
	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"

	"github.com/VictoriaMetrics/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	version = "dev"
	date    = "unknown"
)

const serverName = "mcp-vmanomaly"

const serverInstructions = `When a user wants to optimize business-facing anomaly controls, especially detection_direction away from "both", min_dev_from_expected, or min_rel_dev_from_expected, show the effective guardrails after the resulting anomaly detection task finishes. In VMUI Copilot, enable the "Business boundaries" overlay by applying show_business_boundaries=true through anomaly UI state. Treat show_business_boundaries as a frontend visualization setting, not a model parameter, and preserve the other anomaly UI settings. VMUI query state and suggest_query_config expose only the query expression and language, so keep data_range, detection_direction, min_dev_from_expected, and min_rel_dev_from_expected in suggest_model_config changes; do not try to apply reader.workers through a UI suggestion card. Only when producing a complete vmanomaly v1.30.2+ deployment configuration outside the VMUI suggestion flow, place these stable policies under reader.queries.<alias>; model-level placement remains a compatibility fallback.`

const (
	shutdownPeriod      = 15 * time.Second
	shutdownHardPeriod  = 3 * time.Second
	readinessDrainDelay = 3 * time.Second
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet(serverName, flag.ContinueOnError)
	flags.SetOutput(stderr)
	showVersion := flags.Bool("version", false, "print version and exit")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "unexpected arguments: %v\n", flags.Args())
		return 2
	}
	if *showVersion {
		_, _ = fmt.Fprintf(stdout, "%s v%s (date: %s)\n", serverName, version, date)
		return 0
	}

	c, err := config.InitConfig()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error initializing config: %v\n", err)
		return 1
	}

	if err := serve(c, stderr); err != nil {
		slog.Error("Server stopped with an error", "error_class", hooks.ErrorClass(err))
		return 1
	}
	return 0
}

func serve(c *config.Config, stderr io.Writer) error {
	var logOutput = os.Stderr
	var logFile *os.File
	if c.LogFile() != "" {
		f, err := os.OpenFile(c.LogFile(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Failed to open log file, using stderr: %v\n", err)
		} else {
			logOutput = f
			logFile = f
		}
	}
	if logFile != nil {
		defer logFile.Close()
	}

	var logLevel slog.Level
	switch c.LogLevel() {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(logOutput, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	if !c.IsStdio() {
		slog.Info("Starting server", "name", serverName, "version", version, "date", date)
	}

	ms := metrics.NewSet()
	mcpServer, err := newMCPServer(c, ms, logLevel <= slog.LevelDebug)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP server: %w", err)
	}

	// Stdio mode - simple execution
	if c.IsStdio() {
		if err := server.ServeStdio(mcpServer); err != nil {
			return fmt.Errorf("failed to serve stdio: %w", err)
		}
		return nil
	}

	// SSE/HTTP mode - full server with graceful shutdown
	var isReady atomic.Bool

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()

	// Metrics endpoint
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		ms.WritePrometheus(w)
		metrics.WriteProcessMetrics(w)
	})

	// Health endpoints
	mux.HandleFunc("/health/liveness", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
	})
	mux.HandleFunc("/health/readiness", func(w http.ResponseWriter, _ *http.Request) {
		if !isReady.Load() {
			http.Error(w, "Not ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Ready\n"))
	})

	// Server mode-specific handlers
	switch c.ServerMode() {
	case "sse":
		slog.Info("Starting server in SSE mode", "addr", c.ListenAddr())
		srv := server.NewSSEServer(mcpServer)
		mux.Handle(srv.CompleteSsePath(), srv.SSEHandler())
		mux.Handle(srv.CompleteMessagePath(), srv.MessageHandler())
	case "http":
		slog.Info("Starting server in HTTP mode", "addr", c.ListenAddr())
		heartBeatOption := server.WithHeartbeatInterval(c.HeartbeatInterval())
		srv := server.NewStreamableHTTPServer(mcpServer, heartBeatOption)
		mux.Handle("/mcp", srv)
	default:
		return fmt.Errorf("unknown server mode %q", c.ServerMode())
	}

	ongoingCtx, stopOngoingGracefully := context.WithCancel(context.Background())
	defer stopOngoingGracefully()
	hs := &http.Server{
		Addr:    c.ListenAddr(),
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context {
			return ongoingCtx
		},
	}

	listener, err := net.Listen("tcp", c.ListenAddr())
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", c.ListenAddr(), err)
	}
	defer listener.Close()
	slog.Info("Server is listening", "addr", c.ListenAddr())

	serveErr := make(chan error, 1)
	go func() {
		if err := hs.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- fmt.Errorf("HTTP server failed: %w", err)
		}
	}()

	isReady.Store(true)
	select {
	case <-rootCtx.Done():
	case err := <-serveErr:
		isReady.Store(false)
		stopOngoingGracefully()
		return err
	}
	stop()
	isReady.Store(false)
	slog.Info("Received shutdown signal, shutting down")

	// Give time for readiness check to propagate
	time.Sleep(readinessDrainDelay)
	slog.Info("Readiness check propagated, waiting for ongoing requests to finish")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownPeriod)
	defer cancel()
	err = hs.Shutdown(shutdownCtx)
	stopOngoingGracefully()
	if err != nil {
		slog.Warn("Failed to wait for ongoing requests to finish, forcing cancellation", "error", err)
		time.Sleep(shutdownHardPeriod)
	}

	slog.Info("Server stopped")
	return nil
}

var errToolUnavailable = errors.New("requested tool is not available")

func newMCPServer(c *config.Config, ms *metrics.Set, enableProtocolLogging bool) (*server.MCPServer, error) {
	toolFilter := server.WithToolFilter(func(_ context.Context, toolsList []mcp.Tool) []mcp.Tool {
		filtered := make([]mcp.Tool, 0, len(toolsList))
		for _, tool := range toolsList {
			if c.IsToolEnabled(tool.Name) {
				filtered = append(filtered, tool)
			}
		}
		return filtered
	})
	toolPolicy := server.WithToolHandlerMiddleware(func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if !c.IsToolEnabled(request.Params.Name) {
				return nil, errToolUnavailable
			}
			return next(ctx, request)
		}
	})

	options := []server.ServerOption{
		server.WithRecovery(),
		server.WithToolCapabilities(true),
		server.WithPromptCapabilities(false),
		server.WithInstructions(serverInstructions),
		server.WithHooks(hooks.New(ms)),
		toolFilter,
		toolPolicy,
	}
	if enableProtocolLogging {
		options = append(options, server.WithLogging())
	}
	if !c.IsResourcesDisabled() {
		options = append(options, server.WithResourceCapabilities(true, false))
	}

	mcpServer := server.NewMCPServer(
		serverName,
		fmt.Sprintf("v%s (date: %s)", version, date),
		options...,
	)
	client := vmanomaly.NewClientWithTimeout(
		c.VmanomalyEndpoint(),
		c.BearerToken(),
		c.CustomHeaders(),
		c.RequestTimeout(),
	)
	tools.RegisterTools(mcpServer, client)
	if !c.IsResourcesDisabled() {
		if err := resources.RegisterDocsResources(mcpServer); err != nil {
			return nil, err
		}
	}
	prompts.RegisterPromptConfigRecommendation(mcpServer)
	return mcpServer, nil
}
