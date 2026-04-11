package cmd

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aixgo-dev/aixgo"
)

func TestParseGoVersion(t *testing.T) {
	tests := []struct {
		in        string
		maj, min  int
		ok        bool
	}{
		{"go1.26.3", 1, 26, true},
		{"go1.26", 1, 26, true},
		{"go1.21.0", 1, 21, true},
		{"1.26.3", 1, 26, true},
		{"", 0, 0, false},
		{"devel", 0, 0, false},
		{"go1.x", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			maj, min, ok := parseGoVersion(tt.in)
			if ok != tt.ok || maj != tt.maj || min != tt.min {
				t.Errorf("parseGoVersion(%q) = (%d,%d,%v), want (%d,%d,%v)",
					tt.in, maj, min, ok, tt.maj, tt.min, tt.ok)
			}
		})
	}
}

func TestCheckGoVersion(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		status doctorStatus
	}{
		{"current", "go1.26.0", statusOK},
		{"future", "go1.30.1", statusOK},
		{"newer major", "go2.0.0", statusOK},
		{"too old", "go1.25.0", statusFail},
		{"way too old", "go1.20.0", statusFail},
		{"unparseable", "devel", statusWarn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkGoVersion(tt.in)
			if got.Status != tt.status {
				t.Errorf("status = %v, want %v (msg=%q)", got.Status, tt.status, got.Message)
			}
		})
	}
}

func TestCheckProviderKeys(t *testing.T) {
	t.Run("none configured is fail", func(t *testing.T) {
		got := checkProviderKeys(nil)
		if got.Status != statusFail {
			t.Errorf("got %v, want fail", got.Status)
		}
	})
	t.Run("one configured is ok", func(t *testing.T) {
		got := checkProviderKeys([]string{"anthropic"})
		if got.Status != statusOK {
			t.Errorf("got %v, want ok", got.Status)
		}
		if !strings.Contains(got.Message, "anthropic") {
			t.Errorf("message %q missing provider name", got.Message)
		}
	})
	t.Run("multiple configured is ok", func(t *testing.T) {
		got := checkProviderKeys([]string{"anthropic", "openai", "bedrock"})
		if got.Status != statusOK {
			t.Errorf("got %v, want ok", got.Status)
		}
	})
}

func TestCheckStateDir(t *testing.T) {
	t.Run("missing is warn", func(t *testing.T) {
		got := checkStateDir(filepath.Join(t.TempDir(), "does-not-exist"))
		if got.Status != statusWarn {
			t.Errorf("got %v, want warn", got.Status)
		}
	})

	t.Run("0700 is ok", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "aixgo")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		got := checkStateDir(dir)
		if got.Status != statusOK {
			t.Errorf("got %v, want ok (msg=%q)", got.Status, got.Message)
		}
	})

	t.Run("0755 is warn", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "aixgo")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		got := checkStateDir(dir)
		if got.Status != statusWarn {
			t.Errorf("got %v, want warn", got.Status)
		}
	})

	t.Run("regular file is fail", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		got := checkStateDir(path)
		if got.Status != statusFail {
			t.Errorf("got %v, want fail", got.Status)
		}
	})
}

func TestCheckConfigFile(t *testing.T) {
	t.Run("missing is fail", func(t *testing.T) {
		_, got := checkConfigFile(filepath.Join(t.TempDir(), "absent.yaml"))
		if got.Status != statusFail {
			t.Errorf("got %v, want fail", got.Status)
		}
	})

	t.Run("valid yaml is ok", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ok.yaml")
		body := "agents:\n  - name: a\n    role: logger\n"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, got := checkConfigFile(path)
		if got.Status != statusOK {
			t.Errorf("got %v (msg=%q), want ok", got.Status, got.Message)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		if len(cfg.Agents) != 1 {
			t.Errorf("agents = %d, want 1", len(cfg.Agents))
		}
	})

	t.Run("invalid yaml is fail", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.yaml")
		if err := os.WriteFile(path, []byte("agents: [\n  - broken"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, got := checkConfigFile(path)
		if got.Status != statusFail {
			t.Errorf("got %v, want fail", got.Status)
		}
		if cfg != nil {
			t.Error("expected nil config on parse failure")
		}
	})
}

func TestCheckMCPReachability(t *testing.T) {
	t.Run("empty is nil", func(t *testing.T) {
		got := checkMCPReachability(context.Background(), nil)
		if got != nil {
			t.Errorf("got %d checks, want nil", len(got))
		}
	})

	t.Run("local transport skipped", func(t *testing.T) {
		got := checkMCPReachability(context.Background(), []aixgo.MCPServerDef{
			{Name: "local", Transport: "local"},
		})
		if len(got) != 1 || got[0].Status != statusOK {
			t.Errorf("got %+v, want single ok", got)
		}
	})

	t.Run("grpc without address is fail", func(t *testing.T) {
		got := checkMCPReachability(context.Background(), []aixgo.MCPServerDef{
			{Name: "bad", Transport: "grpc"},
		})
		if len(got) != 1 || got[0].Status != statusFail {
			t.Errorf("got %+v, want single fail", got)
		}
	})

	t.Run("grpc unreachable is warn", func(t *testing.T) {
		// Reserve a loopback port then close it so the address is
		// guaranteed to refuse connections on every OS (including CI
		// runners where 127.0.0.1:1 may behave unpredictably).
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := ln.Addr().String()
		if err := ln.Close(); err != nil {
			t.Fatal(err)
		}
		got := checkMCPReachability(context.Background(), []aixgo.MCPServerDef{
			{Name: "dead", Transport: "grpc", Address: addr},
		})
		if len(got) != 1 || got[0].Status != statusWarn {
			t.Errorf("got %+v, want single warn", got)
		}
	})

	t.Run("grpc reachable is ok", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ln.Close() }()
		addr := ln.Addr().String()
		got := checkMCPReachability(context.Background(), []aixgo.MCPServerDef{
			{Name: "live", Transport: "grpc", Address: addr},
		})
		if len(got) != 1 || got[0].Status != statusOK {
			t.Errorf("got %+v, want single ok", got)
		}
	})
}

func TestStatusGlyph(t *testing.T) {
	tests := []struct {
		in   doctorStatus
		want string
	}{
		{statusOK, "[ok]  "},
		{statusWarn, "[warn]"},
		{statusFail, "[fail]"},
		{doctorStatus("unknown"), "[?]   "},
	}
	for _, tt := range tests {
		if got := statusGlyph(tt.in); got != tt.want {
			t.Errorf("statusGlyph(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestDoctorReportJSONEnvelope locks the JSON envelope shape so downstream
// tooling (bug-report paste, CI parsers) can rely on {ok, checks[]} with
// stable field names and statuses.
func TestDoctorReportJSONEnvelope(t *testing.T) {
	r := doctorReport{
		OK: false,
		Checks: []doctorCheck{
			{Name: "go runtime", Status: statusOK, Message: "Go 1.26"},
			{Name: "provider keys", Status: statusFail, Message: "none"},
		},
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var round struct {
		OK     bool `json:"ok"`
		Checks []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if round.OK != false {
		t.Errorf("ok = %v, want false", round.OK)
	}
	if len(round.Checks) != 2 {
		t.Fatalf("checks = %d, want 2", len(round.Checks))
	}
	if round.Checks[0].Status != "ok" || round.Checks[1].Status != "fail" {
		t.Errorf("statuses = %q/%q, want ok/fail",
			round.Checks[0].Status, round.Checks[1].Status)
	}
	if round.Checks[0].Name != "go runtime" {
		t.Errorf("name = %q, want 'go runtime'", round.Checks[0].Name)
	}
}

func TestDoctorReportOK(t *testing.T) {
	tests := []struct {
		name   string
		checks []doctorCheck
		ok     bool
	}{
		{"empty", nil, true},
		{"all ok", []doctorCheck{{Status: statusOK}, {Status: statusOK}}, true},
		{"warn only", []doctorCheck{{Status: statusOK}, {Status: statusWarn}}, true},
		{"one fail", []doctorCheck{{Status: statusOK}, {Status: statusFail}}, false},
		{"only fail", []doctorCheck{{Status: statusFail}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := doctorReportOK(tt.checks); got != tt.ok {
				t.Errorf("got %v, want %v", got, tt.ok)
			}
		})
	}
}
