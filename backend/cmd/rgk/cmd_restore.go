package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/regikeep/rgk/internal/core"
	"github.com/regikeep/rgk/internal/store"
)

// restoreCmd returns the `rgk restore` command.
func restoreCmd() *cobra.Command {
	var from string
	var to string

	cmd := &cobra.Command{
		Use:   "restore <repo:tag>",
		Short: "Restore a cold-archived image to a registry",
		Long: `Decompress and push a cold-archived image back to a container registry.

Source format: s3://bucket/prefix or oci://namespace/bucket.
Target registry is identified by its ID (e.g. ocir-fra).`,
		Example: `  rgk restore myapp:v1.0.0 --from s3://my-bucket/archives --to ocir-fra`,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			imageArg := args[0]
			if from == "" {
				fmt.Fprintln(os.Stderr, "Error: --from is required")
				os.Exit(1)
			}
			if to == "" {
				fmt.Fprintln(os.Stderr, "Error: --to is required")
				os.Exit(1)
			}

			db := mustGetDB()
			defer db.Close()

			// Find image
			_, repo, tag := parseImageRef(imageArg)
			imgs, err := db.ListImages(store.ImageFilter{Search: repo})
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			var imageID string
			for _, img := range imgs {
				if img.Repo == repo && img.Tag == tag {
					imageID = img.ID
					break
				}
			}
			if imageID == "" {
				fmt.Fprintf(os.Stderr, "Error: image '%s' not found in tracking\n", imageArg)
				os.Exit(1)
			}

			// Find most recent archive for this image
			manifest, err := db.GetArchiveByImageRef(imageID)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if manifest == nil {
				fmt.Fprintf(os.Stderr, "Error: no archive found for image '%s'\n", imageArg)
				os.Exit(1)
			}

			silentLog := zerolog.New(io.Discard)
			restSvc := core.NewRestoreService(db, silentLog)

			fmt.Printf("Restoring %s from %s to %s...\n", imageArg, from, to)
			resp, err := restSvc.Restore(core.RestoreRequest{
				ArchiveID:      manifest.ID,
				TargetRegistry: to,
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			for _, step := range resp.Steps {
				icon := colorGreen("✓")
				if step.Status != "complete" {
					icon = colorRed("✗")
				}
				detail := ""
				if step.Detail != "" {
					detail = " (" + step.Detail + ")"
				}
				fmt.Printf("  %s %s [%s]%s\n", icon, step.Step, step.Duration, detail)
			}

			if resp.Success {
				fmt.Printf("%s Restored %s to %s\n", colorGreen("✓"), imageArg, to)
			} else {
				fmt.Fprintln(os.Stderr, colorRed("Restore failed."))
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Source (s3://bucket/prefix or oci://namespace/bucket) (required)")
	cmd.Flags().StringVar(&to, "to", "", "Target registry ID (e.g. ocir-fra) (required)")
	return cmd
}
