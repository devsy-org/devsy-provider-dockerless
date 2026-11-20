package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devsy-org/devsy/pkg/log"
	"github.com/spf13/cobra"
)

func TestProviderLoggingLevels(t *testing.T) {
	for _, tc := range []struct {
		level string
		want  string
	}{
		{"", "info,warn,error"},
		{"error", "error"},
		{"warn", "warn,error"},
		{"info", "info,warn,error"},
		{"debug", "debug,info,warn,error"},
		{"trace", "trace,debug,info,warn,error"},
		{"TRACE", "trace,debug,info,warn,error"},
	} {
		t.Run(tc.level, func(t *testing.T) {
			t.Setenv("DEVSY_LOG_LEVEL", tc.level)
			t.Setenv("DEVSY_LOG_FORMAT", "")
			var output bytes.Buffer
			remove := log.AddSink(&output)
			defer remove()
			root := NewRootCmd()
			root.AddCommand(&cobra.Command{Use: "probe", Run: func(*cobra.Command, []string) {
				log.Trace("trace")
				log.Debug("debug")
				log.Info("info")
				log.Warn("warn")
				log.Error("error")
			}})
			root.SetArgs([]string{"probe"})
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if err := log.Sync(); err != nil {
				t.Fatal(err)
			}
			levels := providerLogLevels(t, output.String())
			if got := strings.Join(levels, ","); got != tc.want {
				t.Fatalf("levels = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProviderLoggingConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, level, format string
		valid               bool
	}{
		{"auto", "", "auto", true},
		{"json", "", "json", true},
		{"text", "", "text", true},
		{"plain", "", "plain", true},
		{"logfmt", "", "logfmt", true},
		{"uppercase", "INFO", "JSON", true},
		{"level whitespace", " info", "json", false},
		{"format whitespace", "info", "json ", false},
		{"invalid level", "secret-invalid-level", "json", false},
		{"invalid format", "info", "secret-invalid-format", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DEVSY_LOG_LEVEL", tc.level)
			t.Setenv("DEVSY_LOG_FORMAT", tc.format)
			ran := false
			root := NewRootCmd()
			root.AddCommand(
				&cobra.Command{Use: "probe", Run: func(*cobra.Command, []string) { ran = true }},
			)
			root.SetArgs([]string{"probe"})
			err := root.Execute()
			if (err == nil) != tc.valid || ran != tc.valid {
				t.Fatalf("error = %v, command ran = %v", err, ran)
			}
			if err != nil && strings.Contains(err.Error(), "secret-invalid") {
				t.Fatalf("invalid value leaked: %v", err)
			}
		})
	}
}

func TestProviderLoggingRedactsEnvironmentSecrets(t *testing.T) {
	t.Setenv("DEVSY_LOG_LEVEL", "info")
	t.Setenv("DEVSY_LOG_FORMAT", "json")
	t.Setenv("API_TOKEN", "provider-secret-canary-123456789")
	var output bytes.Buffer
	remove := log.AddSink(&output)
	defer remove()
	root := NewRootCmd()
	root.AddCommand(&cobra.Command{Use: "probe", Run: func(*cobra.Command, []string) {
		log.Info("useful diagnostic provider-secret-canary-123456789")
	}})
	root.SetArgs([]string{"probe"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if err := log.Sync(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "provider-secret-canary-123456789") ||
		!strings.Contains(output.String(), "useful diagnostic") {
		t.Fatalf("unexpected diagnostic: %s", output.String())
	}
}

func providerLogLevels(t *testing.T, output string) []string {
	t.Helper()
	var levels []string
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("invalid JSON record %q: %v", line, err)
		}
		level, ok := record["level"].(string)
		if !ok || record["msg"] != level || record["ts"] == nil {
			t.Fatalf("invalid record: %v", record)
		}
		levels = append(levels, level)
	}
	return levels
}
