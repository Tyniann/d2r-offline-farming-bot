package app

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/version"
)

const (
	diagnosticMaximumFileBytes = 4 << 20
	diagnosticMaximumFiles     = 100
)

var (
	diagnosticSecretYAML = regexp.MustCompile(`(?im)^(\s*[^#\r\n]*(?:token|secret|password|api[_-]?key|authorization)[^:=\r\n]*\s*[:=]\s*).*$`)
	diagnosticSecretJSON = regexp.MustCompile(`(?i)("(?:[^"])*(?:token|secret|password|api[_-]?key|authorization)(?:[^"])*"\s*:\s*)"[^"]*"`)
	diagnosticUserPath   = regexp.MustCompile(`(?i)[a-z]:[\\/]+Users[\\/]+[^\\/"\r\n]+(?:[\\/][^"\r\n,}]*)?`)
)

// DiagnosticBundleOptions hält die beiden ausdrücklich bestätigten sensitiven Beigaben.
type DiagnosticBundleOptions struct {
	IncludeTelemetry bool
	IncludeRoutes    bool
}

// DiagnosticBundleResult beschreibt ein lokal erzeugtes Paket ohne absoluten Pfad.
type DiagnosticBundleResult struct {
	Filename          string
	Bytes             int64
	IncludedTelemetry bool
	IncludedRoutes    bool
}

// DiagnosticBundleCollector erzeugt ausschließlich allowlist-basierte lokale ZIPs aus einem festen Datenroot.
type DiagnosticBundleCollector struct {
	root    string
	history *telemetry.HistoryIndex
	now     func() time.Time
}

// NewDiagnosticBundleCollector bindet den Collector an einen absoluten installierten Datenroot.
func NewDiagnosticBundleCollector(root string, history *telemetry.HistoryIndex) (*DiagnosticBundleCollector, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("%s: diagnostic data root must be absolute", Phase15ReasonDiagnosticContentRejected)
	}
	return &DiagnosticBundleCollector{root: filepath.Clean(root), history: history, now: time.Now}, nil
}

// Create erzeugt atomar ein lokales ZIP; Telemetrie und Routen bleiben ohne explizites Opt-in ausgeschlossen.
func (c *DiagnosticBundleCollector) Create(options DiagnosticBundleOptions) (DiagnosticBundleResult, error) {
	if c == nil || c.root == "" {
		return DiagnosticBundleResult{}, fmt.Errorf("%s: diagnostic collector is unavailable", Phase15ReasonDiagnosticBundleFailed)
	}
	directory := filepath.Join(c.root, "diagnostics")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return DiagnosticBundleResult{}, fmt.Errorf("%s: create diagnostics directory: %w", Phase15ReasonDiagnosticBundleFailed, err)
	}
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return DiagnosticBundleResult{}, fmt.Errorf("%s: generate diagnostic name: %w", Phase15ReasonDiagnosticBundleFailed, err)
	}
	filename := fmt.Sprintf("diagnose-%s-%s.zip", c.now().UTC().Format("20060102T150405Z"), hex.EncodeToString(suffix))
	finalPath := filepath.Join(directory, filename)
	temporary, err := os.CreateTemp(directory, ".diagnose-*.tmp")
	if err != nil {
		return DiagnosticBundleResult{}, fmt.Errorf("%s: create diagnostic staging file: %w", Phase15ReasonDiagnosticBundleFailed, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	writer := zip.NewWriter(temporary)
	if writeErr := c.writeBundle(writer, options); writeErr != nil {
		_ = writer.Close()
		_ = temporary.Close()
		return DiagnosticBundleResult{}, writeErr
	}
	if closeErr := writer.Close(); closeErr != nil {
		_ = temporary.Close()
		return DiagnosticBundleResult{}, fmt.Errorf("%s: close diagnostic archive: %w", Phase15ReasonDiagnosticBundleFailed, closeErr)
	}
	if syncErr := temporary.Sync(); syncErr != nil {
		_ = temporary.Close()
		return DiagnosticBundleResult{}, fmt.Errorf("%s: sync diagnostic archive: %w", Phase15ReasonDiagnosticBundleFailed, syncErr)
	}
	if fileCloseErr := temporary.Close(); fileCloseErr != nil {
		return DiagnosticBundleResult{}, fmt.Errorf("%s: close diagnostic file: %w", Phase15ReasonDiagnosticBundleFailed, fileCloseErr)
	}
	if renameErr := os.Rename(temporaryPath, finalPath); renameErr != nil {
		return DiagnosticBundleResult{}, fmt.Errorf("%s: publish diagnostic archive: %w", Phase15ReasonDiagnosticBundleFailed, renameErr)
	}
	info, err := os.Stat(finalPath)
	if err != nil {
		return DiagnosticBundleResult{}, fmt.Errorf("%s: stat diagnostic archive: %w", Phase15ReasonDiagnosticBundleFailed, err)
	}
	return DiagnosticBundleResult{Filename: filename, Bytes: info.Size(), IncludedTelemetry: options.IncludeTelemetry, IncludedRoutes: options.IncludeRoutes}, nil
}

func (c *DiagnosticBundleCollector) writeBundle(writer *zip.Writer, options DiagnosticBundleOptions) error {
	manifest, err := json.MarshalIndent(map[string]any{
		"schema_version":     1,
		"generated_at":       c.now().UTC(),
		"core_version":       version.Version,
		"core_commit":        version.Commit,
		"telemetry_included": options.IncludeTelemetry,
		"routes_included":    options.IncludeRoutes,
		"automatic_upload":   false,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: encode diagnostic manifest: %w", Phase15ReasonDiagnosticBundleFailed, err)
	}
	if err := c.writeEntry(writer, "manifest.json", manifest); err != nil {
		return err
	}

	for _, name := range []string{"config.yaml", "operator-settings.local.yaml", "route-lifecycle.local.yaml", "route-assignments.local.yaml", "pickit-assignments.local.yaml", "offsets.scanned.yaml"} {
		if err := c.writeOptionalFile(writer, filepath.Join(c.root, "configs", name), "config/"+name); err != nil {
			return err
		}
	}
	if c.history != nil {
		snapshot := c.history.Snapshot("")
		data, marshalErr := json.MarshalIndent(map[string]any{
			"generation": snapshot.Generation, "updated_at": snapshot.UpdatedAt,
			"diagnostics": snapshot.Diagnostics, "ignored_files": snapshot.IgnoredFiles,
		}, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("%s: encode history diagnostics: %w", Phase15ReasonDiagnosticBundleFailed, marshalErr)
		}
		if err := c.writeEntry(writer, "history/reader-diagnostics.json", data); err != nil {
			return err
		}
	}
	if err := c.writeSelectedDirectory(writer, filepath.Join(c.root, "logs"), "logs", false, func(name string) bool {
		return strings.EqualFold(filepath.Ext(name), ".log")
	}, 3); err != nil {
		return err
	}
	if options.IncludeTelemetry {
		if err := c.writeSelectedDirectory(writer, filepath.Join(c.root, "logs", "telemetry"), "telemetry", false, func(name string) bool {
			return strings.EqualFold(filepath.Ext(name), ".jsonl")
		}, 20); err != nil {
			return err
		}
	}
	if options.IncludeRoutes {
		if err := c.writeSelectedDirectory(writer, filepath.Join(c.root, "configs", "routes"), "routes", true, func(name string) bool {
			extension := filepath.Ext(name)
			return strings.EqualFold(extension, ".yaml") || strings.EqualFold(extension, ".yml")
		}, diagnosticMaximumFiles); err != nil {
			return err
		}
	}
	return nil
}

func (c *DiagnosticBundleCollector) writeOptionalFile(writer *zip.Writer, source, entry string) error {
	data, exists, err := readDiagnosticFile(source)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return c.writeEntry(writer, entry, data)
}

func (c *DiagnosticBundleCollector) writeSelectedDirectory(writer *zip.Writer, root, entryRoot string, recursive bool, accept func(string) bool, maximum int) error {
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: inspect diagnostic source: %w", Phase15ReasonDiagnosticContentRejected, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s: diagnostic source is not a regular directory", Phase15ReasonDiagnosticContentRejected)
	}
	var names []string
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, relativeErr := filepath.Rel(root, path)
		if relativeErr != nil {
			return relativeErr
		}
		if entry.IsDir() {
			if !recursive {
				return filepath.SkipDir
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("reparse directory rejected")
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("reparse file rejected")
		}
		if accept(entry.Name()) {
			names = append(names, relative)
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("%s: scan diagnostic source: %w", Phase15ReasonDiagnosticContentRejected, walkErr)
	}
	sort.Strings(names)
	if len(names) > maximum {
		names = names[len(names)-maximum:]
	}
	for _, name := range names {
		data, exists, readErr := readDiagnosticFile(filepath.Join(root, name))
		if readErr != nil {
			return readErr
		}
		if exists {
			entry := filepath.ToSlash(filepath.Join(entryRoot, name))
			if writeErr := c.writeEntry(writer, entry, data); writeErr != nil {
				return writeErr
			}
		}
	}
	return nil
}

func readDiagnosticFile(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("%s: inspect diagnostic file: %w", Phase15ReasonDiagnosticContentRejected, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > diagnosticMaximumFileBytes {
		return nil, false, fmt.Errorf("%s: diagnostic file is not an allowed regular file", Phase15ReasonDiagnosticContentRejected)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("%s: read diagnostic file: %w", Phase15ReasonDiagnosticBundleFailed, err)
	}
	return data, true, nil
}

func (c *DiagnosticBundleCollector) writeEntry(writer *zip.Writer, name string, data []byte) error {
	clean := filepath.ToSlash(filepath.Clean(name))
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return fmt.Errorf("%s: invalid diagnostic entry", Phase15ReasonDiagnosticContentRejected)
	}
	entry, err := writer.CreateHeader(&zip.FileHeader{Name: clean, Method: zip.Deflate})
	if err != nil {
		return fmt.Errorf("%s: create diagnostic entry: %w", Phase15ReasonDiagnosticBundleFailed, err)
	}
	redacted := c.redact(data)
	if _, err := io.Copy(entry, bytes.NewReader(redacted)); err != nil {
		return fmt.Errorf("%s: write diagnostic entry: %w", Phase15ReasonDiagnosticBundleFailed, err)
	}
	return nil
}

func (c *DiagnosticBundleCollector) redact(data []byte) []byte {
	text := string(data)
	text = strings.ReplaceAll(text, c.root, "<redacted-path>")
	text = strings.ReplaceAll(text, filepath.ToSlash(c.root), "<redacted-path>")
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		text = strings.ReplaceAll(text, home, "<redacted-path>")
		text = strings.ReplaceAll(text, filepath.ToSlash(home), "<redacted-path>")
	}
	text = diagnosticUserPath.ReplaceAllString(text, "<redacted-path>")
	text = diagnosticSecretYAML.ReplaceAllString(text, `${1}<redacted>`)
	text = diagnosticSecretJSON.ReplaceAllString(text, `${1}"<redacted>"`)
	return []byte(text)
}
