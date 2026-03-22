package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/regikeep/rgk/internal/store"
)

// groupCmd returns the `rgk group` command subtree.
func groupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage keepalive groups",
		Long:  "Create, list, enable, disable, and delete image groups that share a keepalive policy.",
	}
	cmd.AddCommand(
		groupCreateCmd(),
		groupListCmd(),
		groupDeleteCmd(),
		groupEnableCmd(),
		groupDisableCmd(),
	)
	return cmd
}

func groupCreateCmd() *cobra.Command {
	var interval string
	var strategy string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new group",
		Long: `Create a named group of images that share a keepalive interval and strategy.

Interval sets how often the daemon runs keepalive for images in this group:
  7d   = every 7 days      24h  = every 24 hours
  3d   = every 3 days      12h  = every 12 hours

Strategy controls HOW images are kept alive in the registry:
  pull     Pull the image manifest (resets the "last pulled" timestamp).
           Use when the registry deletes images based on inactivity.
  retag    Re-tag the image with the same tag (resets age-based timers).
           Use when the registry deletes images based on age since push.
  native   Apply registry-native protection (lock, pin, or exemption rule).
           Preferred when the registry supports it (e.g. OCIR retention exempt).`,
		Example: `  rgk group create production --interval 7d --strategy pull
  rgk group create staging --interval 24h --strategy retag
  rgk group create critical --interval 3d --strategy native`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			db := mustGetDB()
			defer db.Close()

			g, err := db.CreateGroup(name, interval, strategy)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Printf("Group '%s' created (interval: %s, strategy: %s, id: %s)\n",
				g.Name, g.Interval, g.Strategy, g.ID)
		},
	}

	cmd.Flags().StringVar(&interval, "interval", "7d", "How often to run keepalive (e.g. 7d, 24h, 12h)")
	cmd.Flags().StringVar(&strategy, "strategy", "pull", "How to keep images alive: pull, retag, or native")
	return cmd
}

func groupListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all groups",
		Long:    "Display all keepalive groups with their settings and image counts.",
		Example: `  rgk group list`,
		Run: func(cmd *cobra.Command, args []string) {
			db := mustGetDB()
			defer db.Close()

			groups, err := db.ListGroups()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if len(groups) == 0 {
				fmt.Println("No groups found. Use 'rgk group create' to add one.")
				return
			}

			counts := make(map[string]int)
			for _, g := range groups {
				imgs, err := db.ListImages(store.ImageFilter{GroupID: g.ID})
				if err == nil {
					counts[g.ID] = len(imgs)
				}
			}

			tw := tableWriter()
			fmt.Fprintln(tw, "NAME\tINTERVAL\tSTRATEGY\tENABLED\tIMAGES\tCREATED")
			for _, g := range groups {
				enabled := "yes"
				if !g.Enabled {
					enabled = "no"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\n",
					g.Name, g.Interval, g.Strategy, enabled,
					counts[g.ID],
					g.CreatedAt.Format("2006-01-02"))
			}
			tw.Flush()
		},
	}
}

func groupDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a group",
		Long:  "Delete a group. Images in the group will be unassigned but not deleted.",
		Example: `  rgk group delete staging
  rgk group delete production --force`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			db := mustGetDB()
			defer db.Close()

			g, err := db.GetGroupByName(name)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if g == nil {
				fmt.Fprintf(os.Stderr, "Error: group '%s' not found\n", name)
				os.Exit(1)
			}

			imgs, err := db.ListImages(store.ImageFilter{GroupID: g.ID})
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if len(imgs) > 0 && !force {
				fmt.Fprintf(os.Stderr, "Error: group has %d images. Use --force to delete anyway.\n", len(imgs))
				os.Exit(1)
			}

			if err := db.DeleteGroup(g.ID); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Printf("Group '%s' deleted\n", name)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Delete even if the group has images")
	return cmd
}

func groupEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "enable <name>",
		Short:   "Enable a group",
		Long:    "Enable keepalive scheduling for a group.",
		Example: `  rgk group enable production`,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			db := mustGetDB()
			defer db.Close()

			g, err := db.GetGroupByName(name)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if g == nil {
				fmt.Fprintf(os.Stderr, "Error: group '%s' not found\n", name)
				os.Exit(1)
			}
			if err := db.SetGroupEnabled(g.ID, true); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Printf("Group '%s' enabled\n", name)
		},
	}
}

func groupDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "disable <name>",
		Short:   "Disable a group",
		Long:    "Disable keepalive scheduling for a group without deleting it.",
		Example: `  rgk group disable staging`,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			db := mustGetDB()
			defer db.Close()

			g, err := db.GetGroupByName(name)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if g == nil {
				fmt.Fprintf(os.Stderr, "Error: group '%s' not found\n", name)
				os.Exit(1)
			}
			if err := db.SetGroupEnabled(g.ID, false); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Printf("Group '%s' disabled\n", name)
		},
	}
}
