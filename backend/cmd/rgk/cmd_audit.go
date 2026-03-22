package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/regikeep/rgk/internal/core"
)

// auditCmd returns the `rgk audit` command.
func auditCmd() *cobra.Command {
	var registry string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Scan all tracked images for retention risk",
		Long: `Perform a dry-run retention risk audit of all tracked images.

Images are classified as critical (<=2 days to expiry), warning (<=7 days),
or unpinned (no keepalive group assigned). At-risk images are listed with
recommendations.`,
		Example: `  rgk audit
  rgk audit --registry ocir-fra`,
		Run: func(cmd *cobra.Command, args []string) {
			db := mustGetDB()
			defer db.Close()

			auditSvc := core.NewAuditService(db)
			resp, err := auditSvc.Run(core.AuditRequest{
				DryRun:         true,
				RegistryFilter: registry,
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			fmt.Printf("Scanned %d images. At risk: %d (%d critical, %d warning, %d unpinned)\n",
				resp.Summary.TotalScanned,
				resp.Summary.AtRisk,
				resp.Summary.Critical,
				resp.Summary.Warning,
				resp.Summary.Unpinned,
			)

			if len(resp.Results) == 0 {
				fmt.Println(colorGreen("✓ All clear. No images at risk of deletion."))
				return
			}

			fmt.Println()
			tw := tableWriter()
			fmt.Fprintln(tw, "IMAGE\tREGION\tRISK\tEXPIRES\tRECOMMENDATION")
			for _, r := range resp.Results {
				imageStr := r.Repo + ":" + r.Tag
				expires := fmt.Sprintf("%dd", r.ExpiresIn)
				if r.ExpiresIn < 0 {
					expires = "unknown"
				}

				riskStr := r.Risk
				switch r.Risk {
				case "critical":
					riskStr = colorRed(r.Risk)
				case "warning":
					riskStr = colorYellow(r.Risk)
				default:
					riskStr = colorGray(r.Risk)
				}

				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					imageStr, r.Region, riskStr, expires, r.Recommendation)
			}
			tw.Flush()
		},
	}

	cmd.Flags().StringVar(&registry, "registry", "", "Filter audit to a specific registry ID (e.g. ocir-fra)")
	return cmd
}
