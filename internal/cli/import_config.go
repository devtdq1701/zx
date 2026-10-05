package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"zx/internal/zbxclient"
)

var (
	importFile           string
	importFormat         string
	importUpdateExisting bool
	importCreateMissing  bool
	importDeleteMissing  bool
	importYes            bool

	importConfigCmd = &cobra.Command{
		Use:     "import_configuration",
		Aliases: []string{"import-configuration", "import_config"},
		Short:   "Import configuration (templates, items, triggers, etc.) from YAML, JSON, or XML file",
		Long:    "Import templates and configurations into Zabbix from a local YAML, JSON, or XML file. Validates syntax locally and defaults to dry-run.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if importFile == "" {
				return fmt.Errorf("--file (-f) is required")
			}

			data, err := os.ReadFile(importFile)
			if err != nil {
				return fmt.Errorf("reading configuration file: %w", err)
			}

			fmtChoice := importFormat
			if fmtChoice == "" {
				fmtChoice = zbxclient.DetectFormat(importFile)
			}
			if fmtChoice == "" {
				fmtChoice = "yaml"
			}

			// Pre-validate syntax locally before any server contact
			if err := zbxclient.ValidateLocalFormat(data, fmtChoice); err != nil {
				return err
			}

			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}

			params := zbxclient.ImportConfigParams{
				FilePath:       importFile,
				Source:         string(data),
				Format:         fmtChoice,
				UpdateExisting: importUpdateExisting,
				CreateMissing:  importCreateMissing,
				DeleteMissing:  importDeleteMissing,
			}

			payload, err := client.BuildImportPayload(cmd.Context(), params)
			if err != nil {
				return err
			}

			if !importYes {
				return runMutation(cmd, "configuration.import", payload, false, "")
			}

			_, err = client.ImportConfiguration(cmd.Context(), params)
			if err != nil {
				return fmt.Errorf("import failed: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Configuration imported successfully.")
			return nil
		},
	}
)

func init() {
	importConfigCmd.Flags().StringVarP(&importFile, "file", "f", "", "path to configuration file (required)")
	importConfigCmd.Flags().StringVar(&importFormat, "format", "", "file format (yaml, json, xml; default auto-detected from file extension)")
	importConfigCmd.Flags().BoolVar(&importUpdateExisting, "update-existing", true, "update existing entities")
	importConfigCmd.Flags().BoolVar(&importCreateMissing, "create-missing", true, "create missing entities")
	importConfigCmd.Flags().BoolVar(&importDeleteMissing, "delete-missing", false, "delete missing entities")
	importConfigCmd.Flags().BoolVar(&importYes, "yes", false, "confirm execution (bypasses dry-run)")
	_ = importConfigCmd.MarkFlagRequired("file")
}
