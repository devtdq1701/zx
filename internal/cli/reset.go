package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// resetFlags restores every flag in the command tree to its default. cobra
// keeps parsed values on the package-level variables between Execute calls,
// so a long-lived rootCmd (REPL, tests) would otherwise inherit --yes,
// --profile or --format from the previous command.
func resetFlags(cmd *cobra.Command) {
	reset := func(f *pflag.Flag) {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			var def []string
			if d := strings.Trim(f.DefValue, "[]"); d != "" {
				def = strings.Split(d, ",")
			}
			_ = sv.Replace(def)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	}
	cmd.Flags().VisitAll(reset)
	cmd.PersistentFlags().VisitAll(reset)
	for _, c := range cmd.Commands() {
		resetFlags(c)
	}
}

// executeLine runs one command line on rootCmd from a clean flag state.
func executeLine(ctx context.Context, args []string) error {
	resetFlags(rootCmd)
	rootCmd.SetArgs(args)
	return rootCmd.ExecuteContext(ctx)
}
