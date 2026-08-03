package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	"github.com/zx06/xsql/internal/stats"
)

// StatsFlags holds the flags for the stats command
type StatsFlags struct {
	Profile string
	JSON    bool
}

// StatsLogFlags holds the flags for the stats log subcommand
type StatsLogFlags struct {
	Profile string
	JSON    bool
	Limit   int
}

// NewStatsCommand creates the stats command
func NewStatsCommand(w *output.Writer) *cobra.Command {
	flags := &StatsFlags{}

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show usage statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStats(flags, w)
		},
	}

	cmd.Flags().StringVarP(&flags.Profile, "profile", "p", "", "Filter by profile name")
	cmd.Flags().BoolVar(&flags.JSON, "json", false, "Output as JSON")

	// Add subcommands
	cmd.AddCommand(newStatsLogCommand(w))
	cmd.AddCommand(newStatsResetCommand(w))

	return cmd
}

func runStats(flags *StatsFlags, w *output.Writer) error {
	store := stats.NewStore(GlobalConfig.Stats.FilePath)
	records, err := store.Load()
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to load stats", nil, err)
	}

	filter := stats.Filter{}
	if flags.Profile != "" {
		filter.Profile = flags.Profile
	}

	agg := stats.Query(records, filter)

	format := output.FormatTable
	if flags.JSON {
		format = output.FormatJSON
	}

	return w.WriteOK(format, &stats.StatsResult{Records: agg})
}

func newStatsLogCommand(w *output.Writer) *cobra.Command {
	flags := &StatsLogFlags{}

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show detailed stats log",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatsLog(flags, w)
		},
	}

	cmd.Flags().StringVarP(&flags.Profile, "profile", "p", "", "Filter by profile name")
	cmd.Flags().BoolVar(&flags.JSON, "json", false, "Output as JSON")
	cmd.Flags().IntVar(&flags.Limit, "limit", 100, "Maximum number of records to show")

	return cmd
}

func runStatsLog(flags *StatsLogFlags, w *output.Writer) error {
	store := stats.NewStore(GlobalConfig.Stats.FilePath)
	records, err := store.Load()
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to load stats", nil, err)
	}

	// Apply filter
	filtered := make([]*stats.Record, 0, len(records))
	for _, r := range records {
		if flags.Profile != "" && r.Profile != flags.Profile {
			continue
		}
		filtered = append(filtered, r)
	}

	// Apply limit (from end)
	start := 0
	if len(filtered) > flags.Limit {
		start = len(filtered) - flags.Limit
	}
	filtered = filtered[start:]

	// Convert to Record slice
	result := make([]stats.Record, len(filtered))
	for i, r := range filtered {
		result[i] = *r
	}

	format := output.FormatTable
	if flags.JSON {
		format = output.FormatJSON
	}

	return w.WriteOK(format, &stats.LogResult{
		Records: result,
		Total:   len(records),
	})
}

func newStatsResetCommand(w *output.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset usage statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatsReset(w)
		},
	}
}

func runStatsReset(w *output.Writer) error {
	store := stats.NewStore(GlobalConfig.Stats.FilePath)
	if err := store.Reset(); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to reset stats", nil, err)
	}

	fmt.Fprintln(os.Stderr, "Statistics have been reset.")
	return nil
}

// recordCmdStats records a command execution to the stats store.
func recordCmdStats(cmd string, profile string, ok bool, start time.Duration, errCode errors.Code, sql string) {
	if !GlobalConfig.Stats.Enabled {
		return
	}

	store := stats.NewStore(GlobalConfig.Stats.FilePath)
	r := &stats.Record{
		Timestamp:  time.Now(),
		Cmd:        cmd,
		Profile:    profile,
		OK:         ok,
		DurationMs: start.Milliseconds(),
		Attrs:      GlobalConfig.Attrs,
	}

	if !ok && errCode != "" {
		r.ErrorCode = string(errCode)
	}

	if GlobalConfig.Stats.LogSQL && sql != "" {
		r.SQL = sql
	}

	// Best-effort: don't fail the command if stats recording fails
	_ = store.Append(r)
}
