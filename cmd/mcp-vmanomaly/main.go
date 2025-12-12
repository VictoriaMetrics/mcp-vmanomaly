package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/cmd/mcp-vmanomaly/config"
	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/cmd/mcp-vmanomaly/hooks"
	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/promts"
	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/resources"
	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/tools"
	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/vmanomaly"

	"github.com/VictoriaMetrics/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	version = "dev"
	date    = "unknown"
)

const serverName = "mcp-vmanomaly"

const (
	shutdownPeriod      = 15 * time.Second
	shutdownHardPeriod  = 3 * time.Second
	readinessDrainDelay = 3 * time.Second
)

func main() {
	c, err := config.InitConfig()
	if err != nil {
		fmt.Printf("Error initializing config: %v\n", err)
		return
	}

	var logOutput = os.Stderr
	if c.LogFile() != "" {
		f, err := os.OpenFile(c.LogFile(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open log file, using stderr: %v\n", err)
		} else {
			logOutput = f
			defer f.Close()
		}
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
	client := vmanomaly.NewClient(c.VmanomalyEndpoint(), c.BearerToken(), c.CustomHeaders())

	// Create tool filter that checks disabled tools from config
	toolFilter := server.WithToolFilter(func(_ context.Context, toolsList []mcp.Tool) []mcp.Tool {
		filtered := make([]mcp.Tool, 0, len(toolsList))
		for _, tool := range toolsList {
			if !c.IsToolDisabled(tool.Name) {
				filtered = append(filtered, tool)
			}
		}
		return filtered
	})

	var mcpServer *server.MCPServer
	if logLevel <= slog.LevelDebug {
		mcpServer = server.NewMCPServer(
			serverName,
			fmt.Sprintf("v%s (date: %s)", version, date),
			server.WithRecovery(),
			server.WithLogging(),
			server.WithToolCapabilities(true),
			server.WithResourceCapabilities(!c.IsResourcesDisabled(), false),
			server.WithPromptCapabilities(false),
			server.WithHooks(hooks.New(ms)),
			toolFilter,
		)
	} else {
		mcpServer = server.NewMCPServer(
			serverName,
			fmt.Sprintf("v%s (date: %s)", version, date),
			server.WithRecovery(),
			server.WithToolCapabilities(true),
			server.WithResourceCapabilities(!c.IsResourcesDisabled(), false),
			server.WithPromptCapabilities(false),
			server.WithHooks(hooks.New(ms)),
			toolFilter,
		)
	}

	tools.RegisterTools(mcpServer, client)

	if !c.IsResourcesDisabled() {
		resources.RegisterDocsResources(mcpServer)
	}

	prompts.RegisterPromptConfigRecommendation(mcpServer)

	// Stdio mode - simple execution
	if c.IsStdio() {
		if err := server.ServeStdio(mcpServer); err != nil {
			slog.Error("failed to start server in stdio mode", "error", err)
			os.Exit(1)
		}
		return
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
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write([]byte("OK\n"))
	})
	mux.HandleFunc("/health/readiness", func(w http.ResponseWriter, _ *http.Request) {
		if !isReady.Load() {
			http.Error(w, "Not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
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
		slog.Error("Unknown server mode", "mode", c.ServerMode())
		os.Exit(1)
	}

	ongoingCtx, stopOngoingGracefully := context.WithCancel(context.Background())
	hs := &http.Server{
		Addr:    c.ListenAddr(),
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context {
			return ongoingCtx
		},
	}

	listener, err := net.Listen("tcp", c.ListenAddr())
	if err != nil {
		slog.Error("Failed to listen", "addr", c.ListenAddr(), "error", err)
		os.Exit(1)
	}
	slog.Info("Server is listening", "addr", c.ListenAddr())

	go func() {
		if err := hs.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	isReady.Store(true)
	<-rootCtx.Done()
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
}
