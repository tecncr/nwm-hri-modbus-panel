package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
)

func newConfigCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and validate configuration",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "example",
		Short: "Print an annotated example configuration file",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprint(cmd.OutOrStdout(), config.Example)
		},
	}, &cobra.Command{
		Use:   "validate",
		Short: "Load and validate the configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, path, err := loadConfig(opts)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "configuration OK (file: %s, %d buses, %d devices, storage: %s)\n",
				orNone(path), len(cfg.Buses), len(cfg.Devices), cfg.Storage.Driver)
			return nil
		},
	}, &cobra.Command{
		Use:   "show",
		Short: "Print the effective configuration (secrets redacted)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, _, err := loadConfig(opts)
			if err != nil {
				return err
			}
			if cfg.Storage.TimescaleDB.DSN != "" {
				cfg.Storage.TimescaleDB.DSN = "REDACTED"
			}
			out, err := yaml.Marshal(cfg)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(out)
			return err
		},
	})
	return cmd
}
