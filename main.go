package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	listenAddr           = flag.String("addr", ":8080", "Address to listen on for HTTP requests")
	hostPath             = flag.String("host-path", "/host", "Path where host filesystem is mounted")
	interval             = flag.Duration("scrape-interval", 1*time.Second, "Scrape interval for metrics")
	dockerSocketPath     = flag.String("docker-sock", "/var/run/docker.sock", "Path to Docker socket")
	containerdSocketPath = flag.String("containerd-sock", "/run/containerd/containerd.sock", "Path to containerd socket")
	mode                 = flag.String("mode", "kubernetes", "Container runtime mode: docker or kubernetes")
	logLevel             = flag.String("log-level", "info", "Log level: debug, info, warn, error, none")
)

type Sandbox struct {
	ID        string // Cgroup ID
	Container string // Docker/Kubernetes container name
	Namespace string // Kubernetes only
	Pod       string // Kubernetes only
}

// pollMetrics periodically polls and updates metrics for all sandboxes/containers.
func pollMetrics() {
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		<-ticker.C

		cgroupList, err := getCgroupList()
		if err != nil {
			slog.Error("Failed to get cgroup list", "error", err)
			continue
		}

		if cgroupList == nil {
			slog.Warn("No control groups found")
			continue
		}

		for _, c := range cgroupList {
			updateMetrics(c)
		}
	}
}

// getCgroupList fetches the list of sandboxes/containers based on the mode.
func getCgroupList() ([]Sandbox, error) {
	switch *mode {
	case "kubernetes":
		return listKubernetesPods()
	case "docker":
		return listDockerContainers()
	default:
		return nil, nil
	}
}

// updateMetrics updates all metrics for a single sandbox/container.
func updateMetrics(c Sandbox) {
	cgroup, err := FindCgroup(*hostPath, c.ID)
	if err != nil {
		slog.Warn("Failed to find cgroup", "container", c.Container, "error", err)
		return
	}

	for _, metric := range cgroupMetrics {
		value, err := readMetricValue(cgroup, metric)
		if err != nil {
			slog.Warn("Failed to read cgroup file field", "file", metric.cgroupFile, "field", metric.cgroupFileField, "error", err)
			continue
		}
		slog.Debug("Updating metric value", "container", c.Container, "namespace", c.Namespace, "pod", c.Pod,
			"cgroupFile", metric.cgroupFile, "field", metric.cgroupFileField, "value", value)
		if metric.gauge != nil {
			metric.gauge.WithLabelValues(c.Container, c.Namespace, c.Pod).Set(float64(value))
		} else if metric.counter != nil {
			metric.counter.WithLabelValues(c.Container, c.Namespace, c.Pod).Add(float64(value))
		}
	}
}

// readMetricValue reads the metric value from the cgroup.
func readMetricValue(cgroup *CGroup, metric Metric) (int, error) {
	if metric.cgroupFileField == "" {
		return cgroup.ReadInteger(metric.cgroupFile)
	}
	return cgroup.ReadIntegerField(metric.cgroupFile, metric.cgroupFileField)
}

func parseLogLevel(level *string) slog.Level {
	switch strings.ToLower(*level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "none":
		return slog.Level(999) // Higher than any defined level.
	default:
		slog.Warn("Unknown log level, defaulting to info", "log-level", *level)
		return slog.LevelInfo
	}
}

func main() {
	flag.Parse()

	// Sanity checks

	// Is mode valid?
	if *mode != "docker" && *mode != "kubernetes" {
		slog.Error("Invalid mode specified", "mode", *mode)
		os.Exit(1)
	}

	// Does the host path exist?
	if _, err := os.Stat(*hostPath); os.IsNotExist(err) {
		slog.Error("Host path does not exist", "path", *hostPath)
		os.Exit(1)
	}

	// Does docker socket exist in docker mode?
	if *mode == "docker" {
		if _, err := os.Stat(*dockerSocketPath); os.IsNotExist(err) {
			slog.Error("Docker socket does not exist", "path", *dockerSocketPath)
			os.Exit(1)
		}
	}

	// Does containerd socket exist in kubernetes mode?
	if *mode == "kubernetes" {
		if _, err := os.Stat(*containerdSocketPath); os.IsNotExist(err) {
			slog.Error("Containerd socket does not exist", "path", *containerdSocketPath)
			os.Exit(1)
		}
	}

	slog.SetLogLoggerLevel(parseLogLevel(logLevel))

	slog.Info("Starting cgroup-container-exporter", "mode", *mode, "hostPath", *hostPath, "listenAddr", *listenAddr, "scrapeInterval", *interval)

	go pollMetrics()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", http.RedirectHandler("/metrics", http.StatusFound))

	server := &http.Server{
		Addr:    *listenAddr,
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}
