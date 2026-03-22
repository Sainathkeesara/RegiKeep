package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// daemonCmd returns the `rgk daemon` command subtree.
func daemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Control the keepalive daemon scheduler",
		Long:  "Start, stop, restart, or check the status of the background keepalive scheduler via the rgk server API.",
	}
	cmd.AddCommand(
		daemonStartCmd(),
		daemonStopCmd(),
		daemonStatusCmd(),
		daemonRestartCmd(),
	)
	return cmd
}

func daemonStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "start",
		Short:   "Start the daemon scheduler",
		Long:    "Send a start signal to the rgk server's background scheduler.",
		Example: `  rgk daemon start`,
		Run: func(cmd *cobra.Command, args []string) {
			body, err := apiPost("/api/v1/daemon/start", map[string]string{})
			if err != nil {
				if isConnectionRefused(err) {
					fmt.Fprintln(os.Stderr, "Error: rgk server is not running. Start it with 'rgk serve'.")
				} else {
					fmt.Fprintln(os.Stderr, "Error:", err)
				}
				os.Exit(1)
			}
			fmt.Printf("Daemon started. Response: %s\n", strings.TrimSpace(string(body)))
		},
	}
}

func daemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "stop",
		Short:   "Stop the daemon scheduler",
		Long:    "Send a stop signal to the rgk server's background scheduler.",
		Example: `  rgk daemon stop`,
		Run: func(cmd *cobra.Command, args []string) {
			_, err := apiPost("/api/v1/daemon/stop", map[string]string{})
			if err != nil {
				if isConnectionRefused(err) {
					fmt.Fprintln(os.Stderr, "Error: rgk server is not running. Start it with 'rgk serve'.")
				} else {
					fmt.Fprintln(os.Stderr, "Error:", err)
				}
				os.Exit(1)
			}
			fmt.Println("Daemon stopped.")
		},
	}
}

func daemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show daemon scheduler status",
		Long:    "Query the rgk server for the current state of the background scheduler.",
		Example: `  rgk daemon status`,
		Run: func(cmd *cobra.Command, args []string) {
			body, err := apiGet("/api/v1/daemon/status")
			if err != nil {
				if isConnectionRefused(err) {
					fmt.Fprintln(os.Stderr, "Error: rgk server is not running. Start it with 'rgk serve'.")
				} else {
					fmt.Fprintln(os.Stderr, "Error:", err)
				}
				os.Exit(1)
			}

			var result map[string]interface{}
			if err := json.Unmarshal(body, &result); err != nil {
				// Print raw if not JSON
				fmt.Println(string(body))
				return
			}

			status, _ := result["status"].(string)
			workers, _ := result["workers"].(float64)
			lastRun, _ := result["lastRun"].(string)
			nextRun, _ := result["nextRun"].(string)

			if status == "" {
				status = "unknown"
			}
			if lastRun == "" {
				lastRun = "—"
			}
			if nextRun == "" {
				nextRun = "—"
			}

			fmt.Printf("Status: %s, Workers: %.0f, Last run: %s, Next run: %s\n",
				status, workers, lastRun, nextRun)
		},
	}
}

func daemonRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "restart",
		Short:   "Restart the daemon scheduler",
		Long:    "Stop then start the background scheduler.",
		Example: `  rgk daemon restart`,
		Run: func(cmd *cobra.Command, args []string) {
			_, err := apiPost("/api/v1/daemon/stop", map[string]string{})
			if err != nil {
				if isConnectionRefused(err) {
					fmt.Fprintln(os.Stderr, "Error: rgk server is not running. Start it with 'rgk serve'.")
				} else {
					fmt.Fprintln(os.Stderr, "Error:", err)
				}
				os.Exit(1)
			}
			_, err = apiPost("/api/v1/daemon/start", map[string]string{})
			if err != nil {
				if isConnectionRefused(err) {
					fmt.Fprintln(os.Stderr, "Error: rgk server is not running. Start it with 'rgk serve'.")
				} else {
					fmt.Fprintln(os.Stderr, "Error:", err)
				}
				os.Exit(1)
			}
			fmt.Println("Daemon restarted.")
		},
	}
}

// isConnectionRefused returns true if the error is a TCP connection-refused error.
func isConnectionRefused(err error) bool {
	return strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connect: connection refused") ||
		strings.Contains(err.Error(), "connectex")
}
