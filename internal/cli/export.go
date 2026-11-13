package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/export"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/inventory"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/store"
)

func newExportCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Manage periodic CSV exports",
	}
	regen := &cobra.Command{
		Use:   "regenerate",
		Short: "Rewrite the periodic CSV files of a time range from the database",
		Long: `Rewrite the periodic CSV files whose windows overlap [--from, --to) from the
current database contents, e.g. after stored measurements were corrected.
Files keep their scheduled names, so existing ones are replaced. It is safe to
run while the server is running; no encoder is contacted.

Times are RFC 3339 or, in the export time zone, YYYY-MM-DD or YYYY-MM-DDTHH:MM.`,
		Example: "  nwm-panel export regenerate --from 2026-10-27 --to 2026-11-14",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, _, err := loadConfig(opts)
			if err != nil {
				return err
			}
			log := newLogger(cfg.Log)
			st, err := store.Open(cmd.Context(), cfg.Storage, log)
			if err != nil {
				return err
			}
			defer st.Close()
			if !st.Enabled() {
				return store.ErrDisabled
			}
			// No config path: this command never persists inventory changes.
			inv, err := inventory.New(cfg, "", log)
			if err != nil {
				return err
			}
			defer inv.Close()
			exp, err := export.New(cfg.Export, cfg.Query.MaxExportRows, st, inv, log)
			if err != nil {
				return err
			}
			fromS, _ := cmd.Flags().GetString("from")
			toS, _ := cmd.Flags().GetString("to")
			from, err := parseCLITime(fromS, exp.Location())
			if err != nil {
				return fmt.Errorf("--from: %w", err)
			}
			to, err := parseCLITime(toS, exp.Location())
			if err != nil {
				return fmt.Errorf("--to: %w", err)
			}
			res, err := exp.Regenerate(cmd.Context(), from, to)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(res); err != nil {
				return err
			}
			if len(res.Errors) > 0 {
				return fmt.Errorf("%d export window(s) failed", len(res.Errors))
			}
			return nil
		},
	}
	regen.Flags().String("from", "", "start of the range (required)")
	regen.Flags().String("to", "", "end of the range, exclusive (required)")
	_ = regen.MarkFlagRequired("from")
	_ = regen.MarkFlagRequired("to")
	cmd.AddCommand(regen)
	return cmd
}

func parseCLITime(v string, loc *time.Location) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, v, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised time %q (use RFC 3339, YYYY-MM-DD or YYYY-MM-DDTHH:MM)", v)
}
