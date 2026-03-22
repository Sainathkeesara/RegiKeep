package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/regikeep/rgk/internal/core"
)

// keepaliveCmd returns the `rgk keepalive` command.
func keepaliveCmd() *cobra.Command {
	var group string
	var strategy string

	cmd := &cobra.Command{
		Use:   "keepalive",
		Short: "Manually trigger a keepalive run",
		Long: `Trigger a keepalive operation for all images (or a specific group).

Each image is processed using the configured registry adapter. Results are printed
per-image with duration and any errors. A summary is printed at the end.`,
		Example: `  rgk keepalive --group production
  rgk keepalive --group staging --strategy retag`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg := openConfig()
			db := mustGetDB()
			defer db.Close()

			regMgr := buildRegMgr(cfg)
			silentLog := zerolog.New(io.Discard)
			keepSvc := core.NewKeepaliveService(db, regMgr, silentLog)

			if group == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				os.Exit(1)
			}

			resp, err := keepSvc.Run(core.KeepaliveRequest{
				Group:    group,
				Strategy: strategy,
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			success := 0
			failed := 0
			for _, r := range resp.Results {
				if r.Success {
					success++
					fmt.Printf("%s %s:%s (strategy: %s)\n",
						colorGreen("✓"), r.Repo, r.Tag, r.Strategy)
				} else {
					failed++
					fmt.Printf("%s %s:%s — %s\n",
						colorRed("✗"), r.Repo, r.Tag, r.Error)
				}
			}

			fmt.Printf("\nProcessed %d images. Success: %d, Failed: %d\n",
				resp.Processed, success, failed)
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "Group name to run keepalive for (required)")
	cmd.Flags().StringVar(&strategy, "strategy", "", "Override strategy: pull, retag, or native")
	_ = cmd.MarkFlagRequired("group")
	return cmd
}
