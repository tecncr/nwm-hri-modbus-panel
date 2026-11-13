// Package cli implements the nwm-panel command line.
package cli

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/version"
)

const appName = "nwm-panel"

type options struct {
	configFile string
	v          *viper.Viper
}

// Execute runs the root command.
func Execute() error {
	err := newRoot().Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", appName, err)
	}
	return err
}

func newRoot() *cobra.Command {
	opts := &options{v: config.NewViper()}
	root := &cobra.Command{
		Use:   appName,
		Short: "Modbus master, recorder and dashboard for NWM-HRI encoders",
		Long: appName + ` manages NWM-HRI pulse encoders over Modbus RTU (RS-485 or RTU over TCP),
exposes every register through a REST API, records measurements to SQLite or
TimescaleDB, exports CSV files periodically and serves a web dashboard.

The master API key must be provided in the ` + config.APIKeyEnv + ` environment variable.
Running without a subcommand is equivalent to "` + appName + ` serve".`,
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.PersistentFlags().StringVarP(&opts.configFile, "config", "c", "", "config file (default: first of "+strings.Join(config.SearchPaths(), ", ")+")")

	serve := newServeCmd(opts)
	root.AddCommand(serve, newConfigCmd(opts), newExportCmd(opts), newVersionCmd())
	// Run serve when no subcommand is given, accepting its flags.
	root.Flags().AddFlagSet(serve.Flags())
	root.RunE = serve.RunE

	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return fmt.Errorf("%w\nRun '%s --help' for usage", err, c.CommandPath())
	})
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print detailed build information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.Full(appName))
		},
	}
}

func newLogger(cfg config.LogConfig) *slog.Logger {
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.Level))
	hopts := &slog.HandlerOptions{Level: level}
	var h slog.Handler = slog.NewTextHandler(os.Stderr, hopts)
	if cfg.Format == "json" {
		h = slog.NewJSONHandler(os.Stderr, hopts)
	}
	return slog.New(h)
}
