package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joshp123/cpt-crawl/internal/combustion"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	case "dump":
		return dump(ctx, args[1:])
	case "sessions":
		return sessions(ctx, args[1:])
	case "dump-history":
		return dumpHistory(ctx, args[1:])
	case "window":
		return window(ctx, args[1:])
	case "watch":
		return watch(ctx, args[1:])
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func dump(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("dump", flag.ContinueOnError)
	out := fs.String("out", env("CPT_OUT_DIR", filepath.Join("outputs", time.Now().UTC().Format("20060102T150405Z"))), "output directory")
	serials := fs.String("serials", env("CPT_SERIALS", ""), "comma-separated probe serials")
	apiKey := fs.String("api-key", os.Getenv("CPT_FIREBASE_API_KEY"), "Firebase Web API key")
	authAge := fs.String("auth-age", os.Getenv("CPT_FIREBASE_AUTH_AGE"), "age-encrypted Firebase auth bundle")
	identity := fs.String("identity", os.Getenv("CPT_AGE_IDENTITY"), "age identity")
	if done, err := parseFlags(fs, args); done || err != nil {
		return err
	}
	client, err := combustion.NewClient(ctx, combustion.Config{APIKey: *apiKey, AuthAgePath: *authAge, AgeIdentity: *identity})
	if err != nil {
		return err
	}
	probes, err := client.Probes(ctx, serialSet(*serials))
	if err != nil {
		return err
	}
	if err := requireSelectedProbes(probes, *serials); err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o700); err != nil {
		return err
	}
	var summaries []combustion.ProbeSummary
	var incompleteErr error
	for _, probe := range probes {
		d, chunks, err := client.DumpProbe(ctx, probe)
		if err != nil {
			return fmt.Errorf("dump %s: %w", probe.Serial, err)
		}
		metaPath := filepath.Join(*out, fmt.Sprintf("session-%s-%d.json", probe.Serial, d.Status.SessionID))
		jsonPath := filepath.Join(*out, fmt.Sprintf("session-data-%s-%d.json", probe.Serial, d.Status.SessionID))
		csvPath := filepath.Join(*out, fmt.Sprintf("ProbeData_%s_%d_cloud.csv", probe.Serial, d.Status.SessionID))
		if err := os.WriteFile(metaPath, d.SessionMeta.Raw, 0o600); err != nil {
			return err
		}
		if err := writeJSON(jsonPath, d); err != nil {
			return err
		}
		if err := combustion.WriteCSV(csvPath, d); err != nil {
			return err
		}
		summary := combustion.Summary(d, chunks, jsonPath, csvPath)
		summaries = append(summaries, summary)
		if incompleteErr == nil {
			incompleteErr = incompleteSummaryError(summary)
		}
	}
	summaryPath := filepath.Join(*out, "summary.json")
	if err := writeJSON(summaryPath, summaries); err != nil {
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"out_dir": *out, "summary": summaries}); err != nil {
		return err
	}
	return incompleteErr
}

func sessions(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("sessions", flag.ContinueOnError)
	out := fs.String("out", "", "optional JSON output file")
	serials := fs.String("serials", env("CPT_SERIALS", ""), "comma-separated probe serials")
	apiKey := fs.String("api-key", os.Getenv("CPT_FIREBASE_API_KEY"), "Firebase Web API key")
	authAge := fs.String("auth-age", os.Getenv("CPT_FIREBASE_AUTH_AGE"), "age-encrypted Firebase auth bundle")
	identity := fs.String("identity", os.Getenv("CPT_AGE_IDENTITY"), "age identity")
	if done, err := parseFlags(fs, args); done || err != nil {
		return err
	}
	client, err := combustion.NewClient(ctx, combustion.Config{APIKey: *apiKey, AuthAgePath: *authAge, AgeIdentity: *identity})
	if err != nil {
		return err
	}
	probes, err := client.Probes(ctx, serialSet(*serials))
	if err != nil {
		return err
	}
	if err := requireSelectedProbes(probes, *serials); err != nil {
		return err
	}
	result := map[string][]combustion.Session{}
	for _, probe := range probes {
		items, err := client.Sessions(ctx, probe)
		if err != nil {
			return fmt.Errorf("list sessions for %s: %w", probe.Serial, err)
		}
		result[probe.Serial] = items
	}
	if *out != "" {
		return writeJSON(*out, result)
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func dumpHistory(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("dump-history", flag.ContinueOnError)
	out := fs.String("out", filepath.Join("outputs", "history-"+time.Now().UTC().Format("20060102T150405Z")), "output directory")
	serials := fs.String("serials", env("CPT_SERIALS", ""), "comma-separated probe serials")
	apiKey := fs.String("api-key", os.Getenv("CPT_FIREBASE_API_KEY"), "Firebase Web API key")
	authAge := fs.String("auth-age", os.Getenv("CPT_FIREBASE_AUTH_AGE"), "age-encrypted Firebase auth bundle")
	identity := fs.String("identity", os.Getenv("CPT_AGE_IDENTITY"), "age identity")
	if done, err := parseFlags(fs, args); done || err != nil {
		return err
	}
	client, err := combustion.NewClient(ctx, combustion.Config{APIKey: *apiKey, AuthAgePath: *authAge, AgeIdentity: *identity})
	if err != nil {
		return err
	}
	probes, err := client.Probes(ctx, serialSet(*serials))
	if err != nil {
		return err
	}
	if err := requireSelectedProbes(probes, *serials); err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o700); err != nil {
		return err
	}
	var summaries []combustion.ProbeSummary
	var incompleteErr error
	for _, probe := range probes {
		items, err := client.Sessions(ctx, probe)
		if err != nil {
			return fmt.Errorf("list sessions for %s: %w", probe.Serial, err)
		}
		if err := writeJSON(filepath.Join(*out, "sessions-"+probe.Serial+".json"), items); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, item := range items {
			if seen[item.DeviceSessionID] {
				continue
			}
			seen[item.DeviceSessionID] = true
			sessionID, err := strconv.ParseInt(item.DeviceSessionID, 10, 64)
			if err != nil {
				return fmt.Errorf("parse session id for %s: %w", probe.Serial, err)
			}
			fmt.Fprintf(os.Stderr, "dumping %s session %s\n", probe.Serial, item.DeviceSessionID)
			d, chunks, err := client.DumpSession(ctx, probe, sessionID, item.SamplePeriod)
			if err != nil {
				return fmt.Errorf("dump %s session %s: %w", probe.Serial, item.DeviceSessionID, err)
			}
			dir := filepath.Join(*out, probe.Serial, item.DeviceSessionID)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			metaPath := filepath.Join(dir, "session.json")
			jsonPath := filepath.Join(dir, "session-data.json")
			csvPath := filepath.Join(dir, "ProbeData_"+probe.Serial+"_"+item.DeviceSessionID+"_cloud.csv")
			if err := os.WriteFile(metaPath, d.SessionMeta.Raw, 0o600); err != nil {
				return err
			}
			if err := writeJSON(jsonPath, d); err != nil {
				return err
			}
			if err := combustion.WriteCSV(csvPath, d); err != nil {
				return err
			}
			summary := combustion.Summary(d, chunks, jsonPath, csvPath)
			summaries = append(summaries, summary)
			if incompleteErr == nil {
				incompleteErr = incompleteSummaryError(summary)
			}
		}
	}
	if err := writeJSON(filepath.Join(*out, "summary.json"), summaries); err != nil {
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"out_dir": *out, "summary": summaries}); err != nil {
		return err
	}
	return incompleteErr
}

func window(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("window", flag.ContinueOnError)
	minutes := fs.Float64("minutes", 5, "recent window size in minutes")
	out := fs.String("out", "", "optional JSON output file")
	serials := fs.String("serials", env("CPT_SERIALS", ""), "comma-separated probe serials")
	apiKey := fs.String("api-key", os.Getenv("CPT_FIREBASE_API_KEY"), "Firebase Web API key")
	authAge := fs.String("auth-age", os.Getenv("CPT_FIREBASE_AUTH_AGE"), "age-encrypted Firebase auth bundle")
	identity := fs.String("identity", os.Getenv("CPT_AGE_IDENTITY"), "age identity")
	if done, err := parseFlags(fs, args); done || err != nil {
		return err
	}
	if *minutes <= 0 {
		return fmt.Errorf("--minutes must be greater than 0")
	}
	client, err := combustion.NewClient(ctx, combustion.Config{APIKey: *apiKey, AuthAgePath: *authAge, AgeIdentity: *identity})
	if err != nil {
		return err
	}
	s, err := client.Window(ctx, *minutes, serialSet(*serials))
	if err != nil {
		return err
	}
	if err := requireSelectedProbes(windowProbes(s.Probes), *serials); err != nil {
		return err
	}
	if *out != "" {
		return writeJSON(*out, s)
	}
	return json.NewEncoder(os.Stdout).Encode(s)
}

func watch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	interval := 30 * time.Second
	intervalFlag := durationSecondsValue{target: &interval}
	fs.Var(intervalFlag, "interval", "poll interval")
	fs.Var(intervalFlag, "n", "poll interval")
	serials := fs.String("serials", env("CPT_SERIALS", ""), "comma-separated probe serials")
	apiKey := fs.String("api-key", os.Getenv("CPT_FIREBASE_API_KEY"), "Firebase Web API key")
	authAge := fs.String("auth-age", os.Getenv("CPT_FIREBASE_AUTH_AGE"), "age-encrypted Firebase auth bundle")
	identity := fs.String("identity", os.Getenv("CPT_AGE_IDENTITY"), "age identity")
	if done, err := parseFlags(fs, args); done || err != nil {
		return err
	}
	if interval <= 0 {
		return fmt.Errorf("--interval must be greater than 0")
	}
	client, err := combustion.NewClient(ctx, combustion.Config{APIKey: *apiKey, AuthAgePath: *authAge, AgeIdentity: *identity})
	if err != nil {
		return err
	}
	seen := map[watchKey]bool{}
	last := map[watchKey]int{}
	enc := json.NewEncoder(os.Stdout)
	for {
		s, err := client.Window(ctx, watchLookbackMinutes(interval), serialSet(*serials))
		if err != nil {
			return err
		}
		if err := requireSelectedProbes(windowProbes(s.Probes), *serials); err != nil {
			return err
		}
		for _, record := range watchRecords(s, seen, last) {
			if err := enc.Encode(record); err != nil {
				if isBrokenPipe(err) {
					return nil
				}
				return err
			}
		}
		time.Sleep(interval)
	}
}

func parseFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if len(args) == 1 && args[0] == "help" {
		fs.SetOutput(os.Stdout)
		fs.Usage()
		return true, nil
	}
	if wantsHelp(args) {
		fs.SetOutput(os.Stdout)
	} else {
		fs.SetOutput(os.Stderr)
	}
	err := fs.Parse(args)
	if err == flag.ErrHelp {
		return true, nil
	}
	return false, err
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `usage: cpt-crawl <command> [flags]

Commands:
  sessions       list indexed cloud sessions
  dump           dump current sessions for selected probes
  dump-history   dump unique indexed historical sessions
  window         print recent rows once as JSON
  watch          follow new samples as JSON Lines

Run "cpt-crawl <command> --help" for command flags.`)
}

type durationSecondsValue struct {
	target *time.Duration
}

func (v durationSecondsValue) String() string {
	if v.target == nil {
		return ""
	}
	return v.target.String()
}

func (v durationSecondsValue) Set(s string) error {
	if d, err := time.ParseDuration(s); err == nil {
		*v.target = d
		return nil
	}
	seconds, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("parse duration %q: use 30s or a bare number of seconds", s)
	}
	*v.target = time.Duration(seconds * float64(time.Second))
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func serialSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out[item] = true
		}
	}
	return out
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func requireSelectedProbes(probes []combustion.Probe, serials string) error {
	requested := serialSet(serials)
	if len(requested) == 0 {
		return nil
	}
	found := map[string]bool{}
	for _, probe := range probes {
		found[probe.Serial] = true
	}
	var missing []string
	for serial := range requested {
		if !found[serial] {
			missing = append(missing, serial)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("no probes matched requested serials: %s", strings.Join(missing, ","))
	}
	return nil
}

func windowProbes(probes []combustion.WindowProbe) []combustion.Probe {
	out := make([]combustion.Probe, 0, len(probes))
	for _, probe := range probes {
		out = append(out, combustion.Probe{Serial: probe.Serial})
	}
	return out
}

type watchKey struct {
	Serial    string
	SessionID int64
}

type watchRecord struct {
	GeneratedAt string `json:"generated_at"`
	Serial      string `json:"serial"`
	SessionID   int64  `json:"session_id"`
	combustion.DataRow
}

func watchLookbackMinutes(interval time.Duration) float64 {
	lookback := 2 * time.Minute
	if interval*3 > lookback {
		lookback = interval * 3
	}
	return lookback.Minutes()
}

func watchRecords(s combustion.WindowSnapshot, seen map[watchKey]bool, last map[watchKey]int) []watchRecord {
	var records []watchRecord
	for _, probe := range s.Probes {
		key := watchKey{Serial: probe.Serial, SessionID: probe.SessionID}
		if !seen[key] {
			seen[key] = true
			if probe.Latest != nil {
				records = append(records, watchRecordFor(s.GeneratedAt, probe, *probe.Latest))
				last[key] = probe.Latest.SequenceNumber
			}
			continue
		}
		for _, row := range probe.Rows {
			if row.SequenceNumber <= last[key] {
				continue
			}
			records = append(records, watchRecordFor(s.GeneratedAt, probe, row))
			last[key] = row.SequenceNumber
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].SampledAt == records[j].SampledAt {
			return records[i].Serial < records[j].Serial
		}
		return records[i].SampledAt < records[j].SampledAt
	})
	return records
}

func watchRecordFor(generatedAt string, probe combustion.WindowProbe, row combustion.DataRow) watchRecord {
	return watchRecord{GeneratedAt: generatedAt, Serial: probe.Serial, SessionID: probe.SessionID, DataRow: row}
}

func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}

func incompleteSummaryError(summary combustion.ProbeSummary) error {
	switch {
	case summary.MissingCount > 0:
		return fmt.Errorf("dump %s session %d is incomplete: missing %d rows", summary.Serial, summary.SessionID, summary.MissingCount)
	case summary.Rows != summary.ExpectedRows:
		return fmt.Errorf("dump %s session %d row count mismatch: got %d, expected %d", summary.Serial, summary.SessionID, summary.Rows, summary.ExpectedRows)
	default:
		return nil
	}
}
