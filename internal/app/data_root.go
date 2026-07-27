package app

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
	"gopkg.in/yaml.v3"
)

const (
	dataRootBundleSchemaVersion = 1
	dataRootBundleManifestName  = "bundle.json"
)

var errDataRootSchemaUnsupported = errors.New("data root schema unsupported")

// DataRootStatus beschreibt, ob ein Datenroot neu veröffentlicht oder unverändert wiederverwendet wurde.
type DataRootStatus string

const (
	// DataRootPublished bezeichnet genau einen erfolgreich atomar veröffentlichten Stagingstand.
	DataRootPublished DataRootStatus = "published"
	// DataRootExisting bezeichnet einen bereits vollständig validen Zielroot bei einem Re-Run.
	DataRootExisting DataRootStatus = "existing"
)

// DataRootResult enthält ausschließlich den kanonischen Root und isolierbare History-Diagnosen.
type DataRootResult struct {
	Root        string
	Status      DataRootStatus
	Diagnostics []telemetry.HistoryFileDiagnostic
}

// DataRootError bindet einen stabilen Phase-15-Code an die lokale Fehlerursache.
type DataRootError struct {
	Code Phase15ReasonCode
	Err  error
}

// Error liefert Code und technische Ursache für das oberste strukturierte Log.
func (e *DataRootError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}

// Unwrap erhält die technische Ursache für errors.Is und errors.As.
func (e *DataRootError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// DataRootValidator prüft einen geschlossenen Stagingstand mit produktiven Core-Loadern.
type DataRootValidator func(string) ([]telemetry.HistoryFileDiagnostic, error)

// DataRootManager besitzt genau einen kanonischen Zielroot und veröffentlicht niemals per Merge.
type DataRootManager struct {
	target        string
	validate      DataRootValidator
	rename        func(string, string) error
	beforePublish func(string) error
	reparse       func(string) (bool, error)
}

// NewDataRootManager erstellt einen Manager für einen absoluten, expliziten Zielroot.
func NewDataRootManager(target string) (*DataRootManager, error) {
	if strings.TrimSpace(target) == "" || !filepath.IsAbs(target) {
		return nil, &DataRootError{Code: Phase15ReasonDataRootUnavailable, Err: fmt.Errorf("data root must be absolute")}
	}
	cleanTarget := filepath.Clean(target)
	if filepath.Dir(cleanTarget) == cleanTarget {
		return nil, &DataRootError{Code: Phase15ReasonDataRootUnavailable, Err: fmt.Errorf("data root must not be a filesystem root")}
	}
	return &DataRootManager{
		target:   cleanTarget,
		validate: ValidateInstalledDataRoot,
		rename:   os.Rename,
		reparse:  isReparsePoint,
	}, nil
}

// InitializeDefaults validiert ein read-only Defaultbundle und veröffentlicht daraus einen frischen Root.
func (m *DataRootManager) InitializeDefaults(ctx context.Context, bundleRoot string) (DataRootResult, error) {
	manifest, err := readDefaultBundle(bundleRoot, m.reparse)
	if err != nil {
		if errors.Is(err, errDataRootSchemaUnsupported) {
			return DataRootResult{}, m.failure(Phase15ReasonConfigSchemaUnsupported, err)
		}
		return DataRootResult{}, m.failure(Phase15ReasonDataImportInvalid, err)
	}
	return m.prepare(ctx, Phase15ReasonDataImportFailed, func(staging string) error {
		for _, file := range manifest.Files {
			if err := copyRegularFile(filepath.Join(bundleRoot, filepath.FromSlash(file.Path)), filepath.Join(staging, filepath.FromSlash(file.Path))); err != nil {
				return err
			}
		}
		return nil
	})
}

// Import kopiert einen geschlossenen bestehenden Datenstand ohne Logs, Backups oder Diagnose-ZIPs.
func (m *DataRootManager) Import(ctx context.Context, sourceRoot string) (DataRootResult, error) {
	if strings.TrimSpace(sourceRoot) == "" || !filepath.IsAbs(sourceRoot) {
		return DataRootResult{}, m.failure(Phase15ReasonDataImportInvalid, fmt.Errorf("import root must be absolute"))
	}
	source := filepath.Clean(sourceRoot)
	if samePath(source, m.target) {
		return DataRootResult{}, m.failure(Phase15ReasonDataImportConflict, fmt.Errorf("source and target data roots are identical"))
	}
	if err := ensureNoReparseTree(source, m.reparse); err != nil {
		return DataRootResult{}, m.failure(Phase15ReasonDataImportInvalid, err)
	}
	return m.prepare(ctx, Phase15ReasonDataImportFailed, func(staging string) error {
		if err := copyRegularTree(filepath.Join(source, "configs"), filepath.Join(staging, "configs"), m.reparse); err != nil {
			return fmt.Errorf("copy imported configs: %w", err)
		}
		telemetrySource := filepath.Join(source, "logs", "telemetry")
		entries, err := os.ReadDir(telemetrySource)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("read imported telemetry: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".jsonl" {
				continue
			}
			path := filepath.Join(telemetrySource, entry.Name())
			reparse, checkErr := m.reparse(path)
			if checkErr != nil || reparse || entry.Type()&os.ModeSymlink != 0 {
				if checkErr != nil {
					return checkErr
				}
				return fmt.Errorf("import telemetry %q is a reparse point", entry.Name())
			}
			if err := copyRegularFile(path, filepath.Join(staging, "logs", "telemetry", entry.Name())); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *DataRootManager) prepare(ctx context.Context, failureCode Phase15ReasonCode, stage func(string) error) (result DataRootResult, returnErr error) {
	if m == nil || m.validate == nil || m.rename == nil || m.reparse == nil {
		return DataRootResult{}, &DataRootError{Code: Phase15ReasonDataRootUnavailable, Err: fmt.Errorf("data root manager is incomplete")}
	}
	if err := ctx.Err(); err != nil {
		return DataRootResult{}, m.failure(failureCode, err)
	}
	parent := filepath.Dir(m.target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return DataRootResult{}, m.failure(Phase15ReasonDataRootUnavailable, err)
	}
	if err := ensureNoReparseAncestors(parent, m.reparse); err != nil {
		return DataRootResult{}, m.failure(Phase15ReasonDataRootUnavailable, err)
	}
	if _, err := os.Lstat(m.target); err == nil {
		diagnostics, validationErr := m.validate(m.target)
		if validationErr != nil {
			return DataRootResult{}, m.failure(Phase15ReasonDataImportConflict, fmt.Errorf("existing target is not a valid data root: %w", validationErr))
		}
		return DataRootResult{Root: m.target, Status: DataRootExisting, Diagnostics: diagnostics}, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return DataRootResult{}, m.failure(Phase15ReasonDataRootUnavailable, err)
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(m.target)+".staging-")
	if err != nil {
		return DataRootResult{}, m.failure(Phase15ReasonDataRootUnavailable, err)
	}
	defer func() {
		if staging != "" {
			_ = os.RemoveAll(staging)
		}
	}()
	if stageErr := stage(staging); stageErr != nil {
		return DataRootResult{}, m.failure(failureCode, stageErr)
	}
	if directoryErr := ensureDataRootDirectories(staging); directoryErr != nil {
		return DataRootResult{}, m.failure(failureCode, directoryErr)
	}
	diagnostics, err := m.validate(staging)
	if err != nil {
		return DataRootResult{}, m.failure(Phase15ReasonDataImportInvalid, err)
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return DataRootResult{}, m.failure(failureCode, contextErr)
	}
	if m.beforePublish != nil {
		if publishHookErr := m.beforePublish(staging); publishHookErr != nil {
			return DataRootResult{}, m.failure(failureCode, publishHookErr)
		}
	}
	if publishErr := m.rename(staging, m.target); publishErr != nil {
		return DataRootResult{}, m.failure(failureCode, fmt.Errorf("publish data root: %w", publishErr))
	}
	staging = ""
	postDiagnostics, err := m.validate(m.target)
	if err != nil {
		rollback := filepath.Join(parent, "."+filepath.Base(m.target)+".rollback-"+randomDataRootSuffix())
		// Der Zielroot wurde in diesem Aufruf vollständig erzeugt. Bei einem
		// unerwarteten Fehler im verpflichtenden Re-Read darf er nicht als halb
		// veröffentlichter Root zurückbleiben. Für den Rollback wird bewusst die
		// echte Dateisystemoperation und nicht der Publish-Testhook verwendet.
		if rollbackErr := os.Rename(m.target, rollback); rollbackErr == nil {
			_ = os.RemoveAll(rollback)
		} else if removeErr := os.RemoveAll(m.target); removeErr != nil {
			return DataRootResult{}, m.failure(failureCode, errors.Join(
				fmt.Errorf("verify published data root: %w", err),
				fmt.Errorf("rollback published data root: %w", rollbackErr),
				fmt.Errorf("remove failed published data root: %w", removeErr),
			))
		}
		return DataRootResult{}, m.failure(failureCode, fmt.Errorf("verify published data root: %w", err))
	}
	if len(postDiagnostics) > 0 {
		diagnostics = postDiagnostics
	}
	return DataRootResult{Root: m.target, Status: DataRootPublished, Diagnostics: diagnostics}, nil
}

func (m *DataRootManager) failure(code Phase15ReasonCode, err error) error {
	if m != nil && m.target != "" {
		diagnostic := struct {
			SchemaVersion int               `json:"schema_version"`
			Timestamp     time.Time         `json:"timestamp"`
			Code          Phase15ReasonCode `json:"code"`
			Message       string            `json:"message"`
		}{SchemaVersion: 1, Timestamp: time.Now().UTC(), Code: code, Message: err.Error()}
		if data, marshalErr := json.MarshalIndent(diagnostic, "", "  "); marshalErr == nil {
			_ = os.WriteFile(filepath.Join(filepath.Dir(m.target), filepath.Base(m.target)+"-import-error.json"), append(data, '\n'), 0o600)
		}
	}
	return &DataRootError{Code: code, Err: err}
}

// DefaultBundleManifest beschreibt ausschließlich hashgebundene read-only Defaultdateien.
type DefaultBundleManifest struct {
	SchemaVersion int                 `json:"schema_version"`
	Files         []DefaultBundleFile `json:"files"`
}

// DefaultBundleFile bindet einen relativen Pfad an seinen SHA-256-Inhalt.
type DefaultBundleFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// BuildDefaultBundle erzeugt aus versionierten Config-Assets ein hashgebundenes First-Run-Bundle.
func BuildDefaultBundle(sourceConfigsRoot, bundleRoot string) error {
	if strings.TrimSpace(sourceConfigsRoot) == "" || strings.TrimSpace(bundleRoot) == "" {
		return fmt.Errorf("source configs and bundle root are required")
	}
	if _, err := os.Lstat(bundleRoot); err == nil {
		return fmt.Errorf("default bundle target already exists")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Join(bundleRoot, "configs"), 0o755); err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(bundleRoot)
		}
	}()
	files := [][2]string{
		{"config.example.yaml", "config.yaml"},
		{"offsets.example.yaml", "offsets.example.yaml"},
		{"pickit-assignments.example.yaml", "pickit-assignments.local.yaml"},
	}
	for _, mapping := range files {
		if err := copyRegularFile(filepath.Join(sourceConfigsRoot, mapping[0]), filepath.Join(bundleRoot, "configs", mapping[1])); err != nil {
			return err
		}
	}
	for _, directory := range []string{filepath.Join("pickit", "profiles"), filepath.Join("routes", "town"), "ui"} {
		if err := copyRegularTree(filepath.Join(sourceConfigsRoot, directory), filepath.Join(bundleRoot, "configs", directory), isReparsePoint); err != nil {
			return err
		}
	}
	manifest, err := manifestForBundle(bundleRoot)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(bundleRoot, dataRootBundleManifestName), append(data, '\n'), 0o600); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// ValidateInstalledDataRoot wendet Config-, Route-, Pickit- und History-Loader auf einen Stagingstand an.
func ValidateInstalledDataRoot(root string) ([]telemetry.HistoryFileDiagnostic, error) {
	if strings.TrimSpace(root) == "" || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("installed data root must be absolute")
	}
	if err := ensureNoReparseTree(root, isReparsePoint); err != nil {
		return nil, err
	}
	cfg, err := config.LoadFromDataRoot(root)
	if err != nil {
		return nil, err
	}
	// Die fehlende Operator-Datei wird als einmalige Migration aus den
	// validierten Config-Defaults noch im Staging erzeugt. Ein vorhandener
	// Vertrag durchläuft denselben strikten Loader wie beim Core-Start.
	if _, _, settingsErr := OpenOperatorSettings(cfg); settingsErr != nil {
		return nil, fmt.Errorf("validate operator settings: %w", settingsErr)
	}
	for _, candidate := range []string{
		cfg.ResolvePath(cfg.Routes.FarmingRoot), cfg.ResolvePath(cfg.Routes.CandidateRoot),
		cfg.ResolvePath(cfg.Routes.LifecycleFile), cfg.ResolvePath(cfg.Routes.AssignmentsFile),
		cfg.ResolvePath(cfg.Routes.RecoveryFile), cfg.ResolvePath("pickit/profiles"),
		cfg.ResolvePath("pickit-assignments.local.yaml"), cfg.Telemetry.Directory,
	} {
		if !pathInsideOrEqual(candidate, root) {
			return nil, fmt.Errorf("configured path %q escapes installed data root", candidate)
		}
	}
	if cfg.Memory.OffsetsFile != "" {
		if _, offsetErr := memory.LoadOffsetSetFile(cfg.ResolvePath(cfg.Memory.OffsetsFile)); offsetErr != nil {
			return nil, fmt.Errorf("validate offsets: %w", offsetErr)
		}
	}
	profiles, err := NewPickitProfileService(cfg.ResolvePath("pickit/profiles"))
	if err != nil {
		return nil, err
	}
	profileList, err := profiles.List()
	if err != nil || len(profileList) == 0 {
		if err == nil {
			err = fmt.Errorf("at least one pickit profile is required")
		}
		return nil, err
	}
	assignments, err := NewPickitAssignmentStore(cfg.ResolvePath("pickit-assignments.local.yaml"), profiles)
	if err != nil {
		return nil, err
	}
	if _, snapshotErr := assignments.Snapshot(); snapshotErr != nil {
		return nil, snapshotErr
	}
	routeAssignments, err := NewRouteAssignmentStore(cfg)
	if err != nil {
		return nil, err
	}
	if _, snapshotErr := routeAssignments.Snapshot(); snapshotErr != nil {
		return nil, snapshotErr
	}
	lifecycle, err := NewRouteLifecycleStore(cfg)
	if err != nil {
		return nil, err
	}
	_, catalog, err := lifecycle.Snapshot()
	if err != nil {
		return nil, err
	}
	for _, route := range catalog.Entries {
		if route.Status == RouteLifecycleUnavailable {
			return nil, fmt.Errorf("invalid farming route %q: %s", route.ID, route.Reason)
		}
	}
	candidates, err := NewCandidateStore(cfg)
	if err != nil {
		return nil, err
	}
	if _, listErr := candidates.List(); listErr != nil {
		return nil, listErr
	}
	if townErr := validateTownAssets(cfg); townErr != nil {
		return nil, townErr
	}
	index, err := telemetry.NewHistoryIndex(cfg.Telemetry.Directory)
	if err != nil {
		return nil, err
	}
	if err := index.Refresh(); err != nil {
		return nil, err
	}
	return index.Snapshot("").Diagnostics, nil
}

func validateTownAssets(cfg *config.Config) error {
	graphPath := filepath.Join(cfg.ResolvePath(cfg.Town.Hub.RoutesDirectory), "graph.yaml")
	graph, err := town.LoadServiceGraph(graphPath)
	if err != nil {
		return err
	}
	for _, edge := range graph.Edges {
		for _, variant := range edge.Variants {
			path := filepath.Join(filepath.Dir(graphPath), variant.Route)
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			var route pathing.TownRouteFile
			decoder := yaml.NewDecoder(strings.NewReader(string(data)))
			decoder.KnownFields(true)
			if decodeErr := decoder.Decode(&route); decodeErr != nil {
				return fmt.Errorf("parse town route %q: %w", path, decodeErr)
			}
			origin := world.Position{X: route.LayoutOriginX, Y: route.LayoutOriginY}
			if _, routeErr := pathing.LoadLayoutBoundTownRoute(path, edge.ID, variant.Layout, origin); routeErr != nil {
				return routeErr
			}
		}
	}
	waypointRoot := filepath.Join(filepath.Dir(cfg.ResolvePath(cfg.Town.Hub.RoutesDirectory)), "waypoint")
	waypoints, err := os.ReadDir(waypointRoot)
	if err != nil {
		return err
	}
	for _, entry := range waypoints {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".yaml" {
			continue
		}
		if _, err := pathing.LoadNamedTownRoute(filepath.Join(waypointRoot, entry.Name()), "act1-town-waypoint"); err != nil {
			return err
		}
	}
	for _, egress := range cfg.Town.Egress {
		if _, err := town.LoadSystemEgressRoute(filepath.Join(cfg.ResolvePath(egress.RoutesDirectory), town.SystemEgressFilename)); err != nil {
			return err
		}
	}
	return nil
}

func readDefaultBundle(root string, reparse func(string) (bool, error)) (DefaultBundleManifest, error) {
	if strings.TrimSpace(root) == "" || !filepath.IsAbs(root) {
		return DefaultBundleManifest{}, fmt.Errorf("default bundle root must be absolute")
	}
	if err := ensureNoReparseTree(root, reparse); err != nil {
		return DefaultBundleManifest{}, err
	}
	data, err := os.ReadFile(filepath.Join(root, dataRootBundleManifestName))
	if err != nil {
		return DefaultBundleManifest{}, err
	}
	var manifest DefaultBundleManifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if decodeErr := decoder.Decode(&manifest); decodeErr != nil {
		return DefaultBundleManifest{}, decodeErr
	}
	if trailingErr := decoder.Decode(&struct{}{}); trailingErr != io.EOF {
		return DefaultBundleManifest{}, fmt.Errorf("default bundle must contain one JSON object")
	}
	if manifest.SchemaVersion != dataRootBundleSchemaVersion {
		return DefaultBundleManifest{}, fmt.Errorf("%w: default bundle schema %d", errDataRootSchemaUnsupported, manifest.SchemaVersion)
	}
	if len(manifest.Files) == 0 {
		return DefaultBundleManifest{}, fmt.Errorf("default bundle has no files")
	}
	seen := make(map[string]struct{}, len(manifest.Files))
	for index, file := range manifest.Files {
		clean, pathErr := safeRelativeSlashPath(file.Path)
		if pathErr != nil || clean != file.Path || len(file.SHA256) != 64 {
			return DefaultBundleManifest{}, fmt.Errorf("invalid default bundle file %d", index)
		}
		if _, duplicate := seen[file.Path]; duplicate {
			return DefaultBundleManifest{}, fmt.Errorf("duplicate default bundle path %q", file.Path)
		}
		seen[file.Path] = struct{}{}
		hash, hashErr := sha256File(filepath.Join(root, filepath.FromSlash(file.Path)))
		if hashErr != nil || hash != file.SHA256 {
			return DefaultBundleManifest{}, fmt.Errorf("default bundle hash mismatch for %q", file.Path)
		}
	}
	actual, err := manifestForBundle(root)
	if err != nil {
		return DefaultBundleManifest{}, err
	}
	if len(actual.Files) != len(manifest.Files) {
		return DefaultBundleManifest{}, fmt.Errorf("default bundle contains unlisted files")
	}
	return manifest, nil
}

func manifestForBundle(root string) (DefaultBundleManifest, error) {
	manifest := DefaultBundleManifest{SchemaVersion: dataRootBundleSchemaVersion}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == dataRootBundleManifestName {
			return nil
		}
		hash, err := sha256File(path)
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, DefaultBundleFile{Path: relative, SHA256: hash})
		return nil
	})
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	return manifest, err
}

func ensureDataRootDirectories(root string) error {
	for _, relative := range []string{
		filepath.Join("configs", "routes", "farming"), filepath.Join("configs", "routes", "candidates"),
		filepath.Join("logs", "telemetry"), "backups", "diagnostics",
	} {
		if err := os.MkdirAll(filepath.Join(root, relative), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func copyRegularTree(source, destination string, reparse func(string) (bool, error)) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("copy source %q is not a directory", source)
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		reparsePoint, err := reparse(path)
		if err != nil {
			return err
		}
		if reparsePoint || entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("copy source %q contains a reparse point", path)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("copy path escapes source root")
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyRegularFile(path, target)
	})
}

func copyRegularFile(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source %q is not a regular file", source)
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if directoryErr := os.MkdirAll(filepath.Dir(destination), 0o755); directoryErr != nil {
		return directoryErr
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = out.Close()
		}
	}()
	writer := bufio.NewWriterSize(out, 64*1024)
	if _, err := io.Copy(writer, in); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	closed = true
	return nil
}

func ensureNoReparseTree(root string, check func(string) (bool, error)) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		reparse, err := check(path)
		if err != nil {
			return err
		}
		if reparse || entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("path %q is a reparse point", path)
		}
		return nil
	})
}

func ensureNoReparseAncestors(path string, check func(string) (bool, error)) error {
	current := filepath.Clean(path)
	for {
		if _, err := os.Lstat(current); err == nil {
			reparse, checkErr := check(current)
			if checkErr != nil {
				return checkErr
			}
			if reparse {
				return fmt.Errorf("path %q uses a reparse-point ancestor", current)
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}

func safeRelativeSlashPath(path string) (string, error) {
	if path == "" || strings.Contains(path, "\\") || filepath.IsAbs(filepath.FromSlash(path)) {
		return "", fmt.Errorf("path must be a relative slash path")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path escapes bundle root")
	}
	return clean, nil
}

func pathInsideOrEqual(path, root string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func samePath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func randomDataRootSuffix() string {
	var value [6]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(value[:])
}
