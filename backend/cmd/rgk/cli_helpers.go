package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/regikeep/rgk/internal/config"
	"github.com/regikeep/rgk/internal/registry"
	"github.com/regikeep/rgk/internal/store"
)

// openConfig loads and returns the runtime configuration from environment variables.
func openConfig() *config.Config {
	return config.Load()
}

// openDB loads config and opens the SQLite database at cfg.DBPath.
func openDB() (*store.DB, error) {
	cfg := openConfig()
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open database at %s: %w", cfg.DBPath, err)
	}
	return db, nil
}

// mustGetDB opens the database or prints the error and exits.
func mustGetDB() *store.DB {
	db, err := openDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	return db
}

// buildRegMgr creates registry adapters from DB registries first; falls back to env-var-only adapters.
func buildRegMgr(cfg *config.Config) *registry.Manager {
	db, err := openDB()
	if err == nil {
		defer db.Close()
		regs, listErr := db.ListRegistries()
		if listErr == nil && len(regs) > 0 {
			return registry.BuildFromDB(regs, cfg)
		}
	}

	// Fallback: build from env vars only
	regMgr := registry.NewManager()
	if cfg.OCIREndpoint != "" {
		regMgr.Register(registry.NewOCIRAdapter(
			"ocir-fra",
			cfg.OCIREndpoint,
			cfg.OCIRTenancy,
			cfg.OCIRUsername,
			cfg.OCIRAuthToken,
			cfg.OCIRRegion,
			cfg.OCIRCompartmentOCID,
		))
	}
	if cfg.ECRAccountID != "" {
		regMgr.Register(registry.NewECRAdapter("ecr-use1", cfg.ECRAccountID, cfg.AWSRegion))
	}
	if cfg.DockerHubNamespace != "" {
		regMgr.Register(registry.NewDockerHubAdapter("dockerhub", cfg.DockerHubNamespace))
	}
	return regMgr
}

// tableWriter returns a tabwriter writing to stdout with standard padding.
func tableWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
}

// colorEnabled returns true when ANSI colors should be used.
func colorEnabled() bool {
	return os.Getenv("NO_COLOR") == ""
}

func colorGreen(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

func colorYellow(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\033[33m" + s + "\033[0m"
}

func colorRed(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}

func colorGray(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\033[90m" + s + "\033[0m"
}

// statusColor returns the status string with ANSI color applied.
func statusColor(status string) string {
	switch status {
	case "safe":
		return colorGreen(status)
	case "warning":
		return colorYellow(status)
	case "critical":
		return colorRed(status)
	default:
		return colorGray(status)
	}
}

// apiURL returns the base URL for the running rgk server.
func apiURL() string {
	if v := os.Getenv("RGK_API_URL"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

// apiPost POSTs JSON-encoded body to the given path on the rgk server.
func apiPost(path string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	resp, err := http.Post(apiURL()+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// apiGet performs a GET request to the given path on the rgk server.
func apiGet(path string) ([]byte, error) {
	resp, err := http.Get(apiURL() + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
