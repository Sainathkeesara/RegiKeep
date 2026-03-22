package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/regikeep/rgk/internal/store"
)

// statusCmd returns the `rgk status` command.
func statusCmd() *cobra.Command {
	var group string
	var watch bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show keepalive status for tracked images",
		Long: `Display the current keepalive status for all tracked images.

Use --group to filter by a specific group, and --watch to continuously refresh the
display every 30 seconds.`,
		Example: `  rgk status
  rgk status --group production
  rgk status --watch`,
		Run: func(cmd *cobra.Command, args []string) {
			db := mustGetDB()
			defer db.Close()

			printStatus := func() {
				filter := store.ImageFilter{}
				if group != "" {
					grp, err := db.GetGroupByName(group)
					if err != nil {
						fmt.Fprintln(os.Stderr, "Error:", err)
						return
					}
					if grp == nil {
						fmt.Fprintf(os.Stderr, "Error: group '%s' not found\n", group)
						return
					}
					filter.GroupID = grp.ID
				}

				imgs, err := db.ListImages(filter)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
					return
				}

				// Load group names for display
				groups, _ := db.ListGroups()
				groupNames := make(map[string]string)
				for _, g := range groups {
					groupNames[g.ID] = g.Name
				}

				if len(imgs) == 0 {
					fmt.Println("No images tracked. Use 'rgk add' to register an image.")
					return
				}

				tw := tableWriter()
				fmt.Fprintln(tw, "IMAGE\tGROUP\tLAST KEEPALIVE\tNEXT RUN\tSTATUS\tEXPIRES")
				for _, img := range imgs {
					imageStr := img.Repo + ":" + img.Tag

					grpName := ""
					if img.GroupID != nil {
						grpName = groupNames[*img.GroupID]
					}

					lastKA := "never"
					if img.LastKeepaliveAt != nil {
						lastKA = img.LastKeepaliveAt.Format("2006-01-02 15:04")
					}

					nextRun := "—"

					expires := "unknown"
					if img.ExpiresInDays >= 0 {
						expires = fmt.Sprintf("%dd", img.ExpiresInDays)
					}

					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
						imageStr, grpName, lastKA, nextRun, statusColor(img.LastStatus), expires)
				}
				tw.Flush()
			}

			if !watch {
				printStatus()
				return
			}

			// Watch mode: refresh every 30s, handle Ctrl+C cleanly
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			// Print immediately, then loop
			fmt.Print("\033[2J\033[H")
			printStatus()
			fmt.Println("\nRefreshing every 30s. Press Ctrl+C to exit.")

			for {
				select {
				case <-sig:
					fmt.Println("\nExiting.")
					return
				case <-ticker.C:
					fmt.Print("\033[2J\033[H")
					printStatus()
					fmt.Println("\nRefreshing every 30s. Press Ctrl+C to exit.")
				}
			}
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "Filter by group name")
	cmd.Flags().BoolVar(&watch, "watch", false, "Refresh every 30 seconds")
	return cmd
}
