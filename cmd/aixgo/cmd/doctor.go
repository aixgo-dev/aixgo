package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo"
	"github.com/aixgo-dev/aixgo/pkg/llm/provider"
	"github.com/spf13/cobra"
)

// errDoctorChecksFailed is returned by runDoctor when one or more checks
// failed. Cobra's root.Execute() converts any non-nil error into exit 1,
// so returning this sentinel gives us the same exit code as os.Exit(1)
// without bypassing deferred cleanup. It is paired with SilenceErrors /
// SilenceUsage on doctorCmd so cobra does not print a redundant error
// line or usage dump after we have already rendered the report.
var errDoctorChecksFailed = errors.New("doctor: one or more checks failed")

// doctorMinGoMajor and doctorMinGoMinor define the minimum Go version the
// project supports. Keep in sync with go.mod.
const (
	doctorMinGoMajor = 1
	doctorMinGoMinor = 26
)

// doctorDialTimeout bounds the per-server MCP reachability probe so a slow
// or blackholed address cannot hang the doctor run.
const doctorDialTimeout = 2 * time.Second

// doctorStatus is the per-check outcome. warn does not fail the overall run.
type doctorStatus string

const (
	statusOK   doctorStatus = "ok"
	statusWarn doctorStatus = "warn"
	statusFail doctorStatus = "fail"
)

// doctorCheck is the result of a single diagnostic check.
type doctorCheck struct {
	Name    string       `json:"name"`
	Status  doctorStatus `json:"status"`
	Message string       `json:"message"`
}

// doctorReport is the JSON envelope emitted in --output json mode.
type doctorReport struct {
	OK     bool          `json:"ok"`
	Checks []doctorCheck `json:"checks"`
}

var (
	doctorConfigFile string
	doctorOutput     string
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run environment diagnostics and readiness checks",
	// SilenceUsage: we already render a full report, so cobra's usage dump
	// on error would be noise. SilenceErrors: root.Execute prints the error
	// itself; our sentinel is lowercased to match idiomatic cobra output.
	SilenceErrors: true,
	SilenceUsage:  true,
	Long: `Run a fast environment readiness check and report the status of each area.

aixgo doctor is intended as the first command to run after install and as
the thing to paste into bug reports. It verifies:

  - Go runtime version meets the minimum required
  - At least one LLM provider API key is configured
  - ~/.aixgo state directory exists with restrictive permissions
  - Optional: a YAML config file parses cleanly (--config)
  - Optional: every MCP server in the config is reachable (--config)

Exit codes:
  0  all checks pass (warnings are OK)
  1  any check failed

Examples:
  aixgo doctor
  aixgo doctor --output json
  aixgo doctor --config config/agents.yaml
  aixgo doctor --config config/agents.yaml --output json`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)

	doctorCmd.Flags().StringVarP(&doctorConfigFile, "config", "c", getEnv("CONFIG_FILE", ""), "Optional YAML config to validate and probe")
	doctorCmd.Flags().StringVarP(&doctorOutput, "output", "o", "text", "Output format: text, json")

	_ = doctorCmd.RegisterFlagCompletionFunc("config", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"yaml", "yml"}, cobra.ShellCompDirectiveFilterFileExt
	})
	_ = doctorCmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	if doctorOutput != "text" && doctorOutput != "json" {
		return fmt.Errorf("invalid --output %q: must be 'text' or 'json'", doctorOutput)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	checks := runDoctorChecks(ctx, doctorConfigFile)
	report := doctorReport{
		OK:     doctorReportOK(checks),
		Checks: checks,
	}

	switch doctorOutput {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return fmt.Errorf("encode report: %w", err)
		}
	default:
		printDoctorText(report)
	}

	if !report.OK {
		// Returning a sentinel gives cobra's root.Execute the exit-1 it
		// needs while still running our deferred cancel(). SilenceErrors
		// on doctorCmd prevents cobra from double-printing, but the outer
		// Execute in root.go will still emit this message to stderr,
		// which is fine alongside the rendered report.
		return errDoctorChecksFailed
	}
	return nil
}

// runDoctorChecks executes every diagnostic check and returns the results in
// a deterministic order. It is a pure helper (modulo file/env reads) so it
// can be exercised from tests without shelling out.
func runDoctorChecks(ctx context.Context, configPath string) []doctorCheck {
	var checks []doctorCheck

	checks = append(checks, checkGoVersion(runtime.Version()))
	checks = append(checks, checkProviderKeys(provider.GetAvailableProviderNames()))

	home, err := os.UserHomeDir()
	if err != nil {
		checks = append(checks, doctorCheck{
			Name:    "state directory",
			Status:  statusWarn,
			Message: fmt.Sprintf("cannot resolve home directory: %v", err),
		})
	} else {
		checks = append(checks, checkStateDir(filepath.Join(home, ".aixgo")))
	}

	if configPath != "" {
		cfg, cfgCheck := checkConfigFile(configPath)
		checks = append(checks, cfgCheck)
		if cfg != nil {
			checks = append(checks, checkMCPReachability(ctx, cfg.MCPServers)...)
		}
	}

	return checks
}

// checkGoVersion parses a Go version string like "go1.26.3" and reports
// whether it meets the minimum supported version.
func checkGoVersion(v string) doctorCheck {
	major, minor, ok := parseGoVersion(v)
	if !ok {
		return doctorCheck{
			Name:    "go runtime",
			Status:  statusWarn,
			Message: fmt.Sprintf("unrecognized Go version %q", v),
		}
	}
	if major < doctorMinGoMajor || (major == doctorMinGoMajor && minor < doctorMinGoMinor) {
		return doctorCheck{
			Name:   "go runtime",
			Status: statusFail,
			Message: fmt.Sprintf(
				"Go %d.%d < required %d.%d",
				major, minor, doctorMinGoMajor, doctorMinGoMinor,
			),
		}
	}
	return doctorCheck{
		Name:    "go runtime",
		Status:  statusOK,
		Message: fmt.Sprintf("Go %d.%d", major, minor),
	}
}

// parseGoVersion extracts major and minor version numbers from a Go version
// string of the form "go1.26.3" or "go1.26". Returns ok=false when the
// prefix or digits cannot be parsed.
func parseGoVersion(v string) (major, minor int, ok bool) {
	v = strings.TrimPrefix(v, "go")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return maj, min, true
}

// checkProviderKeys reports which LLM providers have API keys configured.
// Zero providers is a fail (the CLI cannot do any LLM work).
func checkProviderKeys(available []string) doctorCheck {
	if len(available) == 0 {
		return doctorCheck{
			Name:   "provider keys",
			Status: statusFail,
			Message: "no provider API keys detected (set one of " +
				"ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY, " +
				"XAI_API_KEY, or AWS credentials for Bedrock)",
		}
	}
	return doctorCheck{
		Name:    "provider keys",
		Status:  statusOK,
		Message: fmt.Sprintf("configured: %s", strings.Join(available, ", ")),
	}
}

// checkStateDir verifies the ~/.aixgo directory exists with mode 0o700.
// Missing is a warn (it will be created on first use); wider perms is a warn.
func checkStateDir(dir string) doctorCheck {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return doctorCheck{
				Name:    "state directory",
				Status:  statusWarn,
				Message: fmt.Sprintf("%s does not exist (will be created on first use)", dir),
			}
		}
		return doctorCheck{
			Name:    "state directory",
			Status:  statusFail,
			Message: fmt.Sprintf("stat %s: %v", dir, err),
		}
	}
	if !info.IsDir() {
		return doctorCheck{
			Name:    "state directory",
			Status:  statusFail,
			Message: fmt.Sprintf("%s is not a directory", dir),
		}
	}
	mode := info.Mode().Perm()
	if mode&0o077 != 0 {
		return doctorCheck{
			Name:    "state directory",
			Status:  statusWarn,
			Message: fmt.Sprintf("%s has permissive mode %04o (expected 0700)", dir, mode),
		}
	}
	return doctorCheck{
		Name:    "state directory",
		Status:  statusOK,
		Message: fmt.Sprintf("%s mode %04o", dir, mode),
	}
}

// checkConfigFile loads and parses the YAML config via the shared secure
// loader, returning the parsed config on success so downstream checks can
// use it. Parse failure is a fail.
func checkConfigFile(path string) (*aixgo.Config, doctorCheck) {
	loader := aixgo.NewConfigLoader(&aixgo.OSFileReader{})
	cfg, err := loader.LoadConfig(path)
	if err != nil {
		return nil, doctorCheck{
			Name:    "config file",
			Status:  statusFail,
			Message: fmt.Sprintf("%s: %v", path, err),
		}
	}
	return cfg, doctorCheck{
		Name:    "config file",
		Status:  statusOK,
		Message: fmt.Sprintf("%s parsed (%d agents, %d mcp servers)", path, len(cfg.Agents), len(cfg.MCPServers)),
	}
}

// checkMCPReachability probes every gRPC-transport MCP server with a short
// TCP dial. Local-transport entries are reported as skipped. Each server
// produces one check.
func checkMCPReachability(ctx context.Context, servers []aixgo.MCPServerDef) []doctorCheck {
	if len(servers) == 0 {
		return nil
	}
	out := make([]doctorCheck, 0, len(servers))
	for _, s := range servers {
		name := fmt.Sprintf("mcp: %s", s.Name)
		if s.Transport != "grpc" {
			out = append(out, doctorCheck{
				Name:    name,
				Status:  statusOK,
				Message: fmt.Sprintf("transport=%s (no reachability probe)", s.Transport),
			})
			continue
		}
		if s.Address == "" {
			out = append(out, doctorCheck{
				Name:    name,
				Status:  statusFail,
				Message: "grpc transport requires address",
			})
			continue
		}
		dialCtx, cancel := context.WithTimeout(ctx, doctorDialTimeout)
		var dialer net.Dialer
		conn, err := dialer.DialContext(dialCtx, "tcp", s.Address)
		cancel()
		if err != nil {
			// Unreachable MCP servers are warnings, not failures: many
			// aixgo commands (chat, models, version) do not need MCP at
			// all, so a dead dev-time MCP endpoint must not exit-1 the
			// whole doctor run. Misconfiguration (missing address) is
			// still a fail above.
			out = append(out, doctorCheck{
				Name:    name,
				Status:  statusWarn,
				Message: fmt.Sprintf("%s: %v", s.Address, err),
			})
			continue
		}
		_ = conn.Close()
		out = append(out, doctorCheck{
			Name:    name,
			Status:  statusOK,
			Message: fmt.Sprintf("%s reachable", s.Address),
		})
	}
	return out
}

// doctorReportOK returns true when no check has statusFail. Warnings are
// informational and do not fail the overall run.
func doctorReportOK(checks []doctorCheck) bool {
	for _, c := range checks {
		if c.Status == statusFail {
			return false
		}
	}
	return true
}

// printDoctorText emits the human-friendly text rendering.
func printDoctorText(r doctorReport) {
	fmt.Println()
	fmt.Println("aixgo doctor")
	fmt.Println("════════════")
	for _, c := range r.Checks {
		fmt.Printf("  %s  %-22s  %s\n", statusGlyph(c.Status), c.Name, c.Message)
	}
	fmt.Println()
	if r.OK {
		fmt.Println("all checks passed")
	} else {
		fmt.Println("one or more checks failed")
	}
}

func statusGlyph(s doctorStatus) string {
	switch s {
	case statusOK:
		return "[ok]  "
	case statusWarn:
		return "[warn]"
	case statusFail:
		return "[fail]"
	default:
		return "[?]   "
	}
}
