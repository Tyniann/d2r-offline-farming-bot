package app

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticBundleRedactsAndRequiresExplicitOptIn(t *testing.T) {
	root := t.TempDir()
	mustDiagnosticWrite(t, filepath.Join(root, "configs", "config.yaml"), "control_token: super-secret\npath: C:\\Users\\Mario\\save\n")
	mustDiagnosticWrite(t, filepath.Join(root, "logs", "d2rbot.log"), `{"authorization":"Bearer secret","path":"C:/Users/Mario/work"}`)
	mustDiagnosticWrite(t, filepath.Join(root, "logs", "telemetry", "run.jsonl"), `{"event":"loot","token":"hidden"}`)
	mustDiagnosticWrite(t, filepath.Join(root, "configs", "routes", "hero", "route.yaml"), "coordinates: [1, 2]\n")

	collector, err := NewDiagnosticBundleCollector(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := collector.Create(DiagnosticBundleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	entries := readDiagnosticZIP(t, filepath.Join(root, "diagnostics", result.Filename))
	all := strings.Join(mapValues(entries), "\n")
	for _, secret := range []string{"super-secret", "Bearer secret", `C:\Users\Mario`, "C:/Users/Mario"} {
		if strings.Contains(all, secret) {
			t.Fatalf("diagnostic contains rejected content %q", secret)
		}
	}
	if _, ok := entries["telemetry/run.jsonl"]; ok {
		t.Fatal("telemetry included without opt-in")
	}
	if _, ok := entries["routes/hero/route.yaml"]; ok {
		t.Fatal("routes included without opt-in")
	}

	result, err = collector.Create(DiagnosticBundleOptions{IncludeTelemetry: true, IncludeRoutes: true})
	if err != nil {
		t.Fatal(err)
	}
	entries = readDiagnosticZIP(t, filepath.Join(root, "diagnostics", result.Filename))
	if _, ok := entries["telemetry/run.jsonl"]; !ok {
		t.Fatal("telemetry opt-in missing")
	}
	if _, ok := entries["routes/hero/route.yaml"]; !ok {
		t.Fatal("route opt-in missing")
	}
}

func TestDiagnosticBundleRejectsSymlinkedAllowlistSource(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	mustDiagnosticWrite(t, filepath.Join(outside, "run.jsonl"), "{}")
	if err := os.MkdirAll(filepath.Join(root, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "logs", "telemetry")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	collector, err := NewDiagnosticBundleCollector(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Create(DiagnosticBundleOptions{IncludeTelemetry: true}); err == nil || !strings.Contains(err.Error(), string(Phase15ReasonDiagnosticContentRejected)) {
		t.Fatalf("expected content rejection, got %v", err)
	}
}

func mustDiagnosticWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readDiagnosticZIP(t *testing.T, path string) map[string]string {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	result := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		stream, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(stream)
		_ = stream.Close()
		if err != nil {
			t.Fatal(err)
		}
		result[file.Name] = string(data)
	}
	return result
}

func mapValues(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}
