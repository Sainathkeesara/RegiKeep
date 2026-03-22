package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/regikeep/rgk/internal/registry"
	"github.com/regikeep/rgk/internal/store"
)

// detectRegistryType guesses the registry type from the endpoint hostname.
func detectRegistryType(endpoint string) string {
	e := strings.ToLower(endpoint)
	switch {
	case strings.HasSuffix(e, ".ocir.io") || strings.Contains(e, "ocir"):
		return "ocir"
	case strings.Contains(e, ".dkr.ecr.") && strings.Contains(e, ".amazonaws.com"):
		return "ecr"
	case e == "docker.io" || e == "registry-1.docker.io" || e == "hub.docker.com" || strings.Contains(e, "docker"):
		return "dockerhub"
	}
	return ""
}

// configCmd returns the `rgk config` command subtree.
func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage registry configurations",
		Long:  "Add registries, update credential sources, list and test configured registries.",
	}
	cmd.AddCommand(
		configAddRegistryCmd(),
		configDeleteRegistryCmd(),
		configSetCredentialCmd(),
		configListCmd(),
		configTestCmd(),
	)
	return cmd
}

func configAddRegistryCmd() *cobra.Command {
	var regType string
	var region string
	var tenancy string
	var name string
	var username string
	var token string
	var extra string

	cmd := &cobra.Command{
		Use:   "add-registry <endpoint>",
		Short: "Register a new container registry",
		Long: `Add a container registry to RegiKeep.

The endpoint is the registry hostname. The type is auto-detected from the
endpoint but can be overridden with --type.

  *.ocir.io                    → ocir
  *.dkr.ecr.*.amazonaws.com   → ecr
  docker.io                    → dockerhub

Credentials are stored in the database. Pass them with --username and --token.
The --extra flag is for additional auth info (OCIR compartment OCID or ECR account ID).`,
		Example: `  rgk config add-registry fra.ocir.io --region eu-frankfurt-1 --tenancy mytenancy \
      --username 'tenancy/user@example.com' --token 'authtoken' --extra 'ocid1.compartment...'
  rgk config add-registry docker.io --username myuser --token 'dckr_pat_xxx'
  rgk config add-registry 123456789.dkr.ecr.us-east-1.amazonaws.com --region us-east-1 \
      --username 'AKIAXXXXXXXX' --token 'secretkey' --extra '123456789'`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			endpoint := args[0]

			// Auto-detect type if not specified
			if regType == "" {
				regType = detectRegistryType(endpoint)
			}

			switch regType {
			case "ocir", "ecr", "dockerhub":
				// valid
			case "":
				fmt.Fprintln(os.Stderr, "Error: could not detect registry type from endpoint.")
				fmt.Fprintln(os.Stderr, "Use --type ocir, --type ecr, or --type dockerhub.")
				os.Exit(1)
			default:
				fmt.Fprintf(os.Stderr, "Error: invalid --type '%s'. Use ocir, ecr, or dockerhub.\n", regType)
				os.Exit(1)
			}

			db := mustGetDB()
			defer db.Close()

			// Check for duplicate endpoint
			existing, _ := db.GetRegistryByEndpoint(endpoint)
			if existing != nil {
				fmt.Fprintf(os.Stderr, "Error: registry with endpoint '%s' already exists (name: %s)\n", endpoint, existing.Name)
				os.Exit(1)
			}

			registryName := name
			if registryName == "" {
				registryName = endpoint
			}

			credSource := "db"
			if username == "" && token == "" {
				credSource = "env"
			}

			r, err := db.CreateRegistry(store.RegistryConfig{
				Name:             registryName,
				RegistryType:     regType,
				Endpoint:         endpoint,
				Region:           region,
				Tenancy:          tenancy,
				CredentialSource: credSource,
				AuthUsername:     username,
				AuthToken:        token,
				AuthExtra:        extra,
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			fmt.Printf("Registry added:\n")
			fmt.Printf("  Name:     %s\n", r.Name)
			fmt.Printf("  Type:     %s\n", r.RegistryType)
			fmt.Printf("  Endpoint: %s\n", r.Endpoint)
			fmt.Printf("  Region:   %s\n", r.Region)
			fmt.Printf("  Creds:    %s\n", r.CredentialSource)
			fmt.Printf("  ID:       %s\n", r.ID)
			fmt.Println()
			fmt.Println("Run 'rgk config test' to verify connectivity.")
		},
	}

	cmd.Flags().StringVar(&regType, "type", "", "Registry type: ocir, ecr, or dockerhub (auto-detected from endpoint)")
	cmd.Flags().StringVar(&region, "region", "", "Registry region (e.g. us-ashburn-1, us-east-1)")
	cmd.Flags().StringVar(&tenancy, "tenancy", "", "OCI tenancy namespace (OCIR only)")
	cmd.Flags().StringVar(&name, "name", "", "Friendly name (defaults to endpoint)")
	cmd.Flags().StringVar(&username, "username", "", "Auth username (OCIR user, DockerHub user, ECR access key ID)")
	cmd.Flags().StringVar(&token, "token", "", "Auth token (OCIR auth token, DockerHub PAT, ECR secret key)")
	cmd.Flags().StringVar(&extra, "extra", "", "Extra auth info (OCIR compartment OCID, ECR account ID)")
	return cmd
}

func configDeleteRegistryCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete-registry <endpoint>",
		Short:   "Remove a configured registry",
		Example: `  rgk config delete-registry iad.ocir.io`,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			endpoint := args[0]
			db := mustGetDB()
			defer db.Close()

			r, err := db.GetRegistryByEndpoint(endpoint)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if r == nil {
				fmt.Fprintf(os.Stderr, "Error: no registry found with endpoint '%s'\n", endpoint)
				os.Exit(1)
			}

			if err := db.DeleteRegistry(r.ID); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Printf("Registry '%s' (%s) deleted.\n", r.Name, r.Endpoint)
		},
	}
}

func configSetCredentialCmd() *cobra.Command {
	var source string

	cmd := &cobra.Command{
		Use:   "set-credential <endpoint>",
		Short: "Set the credential source for a registry",
		Long: `Update how RegiKeep authenticates with a registry.

Valid sources: env (environment variables), docker (docker credential store), k8s (Kubernetes secret).`,
		Example: `  rgk config set-credential iad.ocir.io --source env
  rgk config set-credential my-registry.example.com --source docker`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			host := args[0]

			switch source {
			case "env", "docker", "k8s":
				// valid
			default:
				fmt.Fprintf(os.Stderr, "Error: invalid source '%s'. Use env, docker, or k8s.\n", source)
				os.Exit(1)
			}

			db := mustGetDB()
			defer db.Close()

			r, err := db.GetRegistryByEndpoint(host)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if r == nil {
				fmt.Fprintf(os.Stderr, "Error: no registry found with endpoint '%s'\n", host)
				os.Exit(1)
			}

			if err := db.UpdateRegistryCredential(r.ID, source); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Printf("Credential source for %s set to '%s'\n", host, source)
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "Credential source: env, docker, or k8s (required)")
	_ = cmd.MarkFlagRequired("source")
	return cmd
}

func configListCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List configured registries",
		Long:    "Display all registered registry endpoints and their configuration.",
		Example: `  rgk config list`,
		Run: func(cmd *cobra.Command, args []string) {
			db := mustGetDB()
			defer db.Close()

			regs, err := db.ListRegistries()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if len(regs) == 0 {
				fmt.Println("No registries configured. Use 'rgk config add-registry' to add one.")
				return
			}

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(regs)
				return
			}

			tw := tableWriter()
			fmt.Fprintln(tw, "NAME\tTYPE\tENDPOINT\tREGION\tCREDENTIALS\tCREATED")
			for _, r := range regs {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					r.Name, r.RegistryType, r.Endpoint, r.Region, r.CredentialSource,
					r.CreatedAt.Format("2006-01-02"))
			}
			tw.Flush()
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

func configTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test [endpoint]",
		Short: "Test connectivity to configured registries",
		Long: `Authenticate against each configured registry and report success or failure.

If an endpoint is given, only that registry is tested.
Otherwise all configured registries are tested.

Credentials must be set in environment variables before running this command.`,
		Example: `  rgk config test
  rgk config test iad.ocir.io`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := mustGetDB()
			defer db.Close()

			cfg := openConfig()

			var regs []store.RegistryConfig
			if len(args) == 1 {
				r, err := db.GetRegistryByEndpoint(args[0])
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
					os.Exit(1)
				}
				if r == nil {
					fmt.Fprintf(os.Stderr, "Error: no registry found with endpoint '%s'\n", args[0])
					os.Exit(1)
				}
				regs = []store.RegistryConfig{*r}
			} else {
				var err error
				regs, err = db.ListRegistries()
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
					os.Exit(1)
				}
				if len(regs) == 0 {
					fmt.Println("No registries configured. Use 'rgk config add-registry' to add one.")
					return
				}
			}

			mgr := registry.BuildFromDB(regs, cfg)
			failed := false

			for _, r := range regs {
				adapter, err := mgr.Get(r.Name)
				if err != nil {
					fmt.Printf("  %s  %s (%s) — %s\n", colorRed("SKIP"), r.Name, r.Endpoint, err)
					failed = true
					continue
				}

				fmt.Printf("  Testing %s (%s, type=%s)... ", r.Name, r.Endpoint, r.RegistryType)
				if err := adapter.Authenticate(); err != nil {
					fmt.Printf("%s\n    %s\n", colorRed("FAIL"), err)
					failed = true
				} else {
					fmt.Printf("%s\n", colorGreen("OK"))
				}
			}

			if failed {
				fmt.Println()
				fmt.Println("Some registries failed. Check:")
				fmt.Println("  - Environment variables are set (OCIR_USERNAME, AWS_ACCESS_KEY_ID, etc.)")
				fmt.Println("  - Network connectivity to the registry endpoint")
				fmt.Println("  - Credentials are valid and not expired")
				os.Exit(1)
			}
		},
	}
}
