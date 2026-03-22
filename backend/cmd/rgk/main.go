package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/regikeep/rgk/internal/api"
	"github.com/regikeep/rgk/internal/config"
	"github.com/regikeep/rgk/internal/store"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	root := &cobra.Command{
		Use:   "rgk",
		Short: "RegiKeep — Container Registry Retention Manager",
	}

	root.AddCommand(
		serveCmd(log),
		versionCmd(),
		groupCmd(),
		addCmd(),
		pushCmd(),
		removeCmd(),
		searchCmd(),
		statusCmd(),
		auditCmd(),
		historyCmd(),
		keepaliveCmd(),
		exportCmd(),
		archiveCmd(),
		restoreCmd(),
		daemonCmd(),
		configCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serveCmd(log zerolog.Logger) *cobra.Command {
	var addr string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the RegiKeep HTTP API server",
		Long: `Start the RegiKeep HTTP API server.

Launches the API on the given address (default :8080) using the
specified SQLite database file. Registry credentials and other
settings are read from environment variables (see .env.example).

Examples:
  rgk serve                        # listen on :8080, DB at /data/regikeep.db
  rgk serve --addr :9090           # listen on port 9090
  rgk serve --db ./local.db        # use a custom database path`,
		RunE: func(c *cobra.Command, args []string) error {
			cfg := config.Load()

			// CLI flags override env vars when explicitly set
			if c.Flags().Changed("addr") {
				cfg.ListenAddr = addr
			}
			if c.Flags().Changed("db") {
				cfg.DBPath = dbPath
			}

			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer db.Close()

			srv := api.NewServer(db, cfg, log)

			// Graceful shutdown
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				if err := srv.Start(); err != nil && err != http.ErrServerClosed {
					log.Fatal().Err(err).Msg("server error")
				}
			}()

			<-quit
			log.Info().Msg("shutting down")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			return srv.Shutdown(ctx)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":8080", "listen address (host:port)")
	cmd.Flags().StringVar(&dbPath, "db", "/data/regikeep.db", "path to SQLite database file")

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("rgk version 0.1.0 (RegiKeep MVP)")
		},
	}
}
