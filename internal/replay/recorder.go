package replay

import (
	"bytes"
	"compress/gzip"
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
	"sync"
	"time"
)

const (
	defaultMaximumFrames      = 12000
	defaultMaximumCheckpoints = 2048
	defaultMaximumBundleBytes = 16 << 20
	maximumExpansionRatio     = 32
	defaultMaximumBundles     = 20
	defaultMaximumTotalBytes  = 128 << 20
)

var traceLabelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)

// ValidateCaptureLabel checks the filesystem-safe operator label used in a
// runtime trace filename.
func ValidateCaptureLabel(label string) error {
	if !traceLabelPattern.MatchString(strings.TrimSpace(label)) {
		return fmt.Errorf("runtime trace label must match %s", traceLabelPattern)
	}
	return nil
}

// Config controls explicit runtime trace capture and bounded local retention.
type Config struct {
	Enabled            bool
	Directory          string
	Label              string
	MaximumFrames      int
	MaximumBundleBytes int64
	MaximumBundles     int
	MaximumTotalBytes  int64
	SaveSuccessful     bool
	Now                func() time.Time
}

// FinalizeResult describes whether and where a terminal trace was persisted.
type FinalizeResult struct {
	Saved    bool
	Filename string
	Bytes    int64
}

// Recorder observes a run. It has no process, window, keyboard, mouse, or task
// dependency and therefore cannot authorize gameplay input.
type Recorder struct {
	mu sync.Mutex

	config    Config
	metadata  Metadata
	contract  ContractSnapshot
	started   time.Time
	frames    []Frame
	current   *Frame
	checks    []Checkpoint
	tick      uint64
	sequence  uint64
	lastStep  string
	closed    bool
	fault     error
	truncated bool

	rename func(string, string) error
}

// NewRecorder creates an opt-in observer. Disabled recorders perform no I/O.
func NewRecorder(config Config, metadata Metadata, contract ContractSnapshot) (*Recorder, error) {
	if !config.Enabled {
		return &Recorder{config: config, closed: true}, nil
	}
	if err := ValidateCaptureLabel(config.Label); err != nil {
		return nil, err
	}
	if strings.TrimSpace(contract.RunID) == "" {
		return nil, fmt.Errorf("runtime trace contract run_id is required")
	}
	if strings.TrimSpace(config.Directory) == "" {
		return nil, fmt.Errorf("runtime trace directory is required")
	}
	if config.MaximumFrames <= 0 {
		config.MaximumFrames = defaultMaximumFrames
	}
	if config.MaximumBundleBytes <= 0 {
		config.MaximumBundleBytes = defaultMaximumBundleBytes
	}
	if config.MaximumBundles <= 0 {
		config.MaximumBundles = defaultMaximumBundles
	}
	if config.MaximumTotalBytes <= 0 {
		config.MaximumTotalBytes = defaultMaximumTotalBytes
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	now := config.Now()
	metadata.Label = config.Label
	metadata.CapturedAt = now.UTC()
	contract.Definition = SanitizeMap(contract.Definition)
	contract.Route = SanitizeMap(contract.Route)
	contract.Policy = SanitizeMap(contract.Policy)
	contract.Loadout = SanitizeMap(contract.Loadout)
	contract.Tuning = SanitizeMap(contract.Tuning)
	return &Recorder{config: config, metadata: metadata, contract: contract, started: now, rename: os.Rename}, nil
}

// Enabled reports whether the recorder is actively observing a run.
func (r *Recorder) Enabled() bool {
	return r != nil && r.config.Enabled && !r.closed
}

// FrameCount reports the number of sealed decision frames retained in memory.
func (r *Recorder) FrameCount() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.frames)
}

// LastFrame returns a defensive copy of the most recently sealed frame.
func (r *Recorder) LastFrame() (Frame, bool) {
	if r == nil {
		return Frame{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.frames) == 0 {
		return Frame{}, false
	}
	encoded, err := json.Marshal(r.frames[len(r.frames)-1])
	if err != nil {
		return Frame{}, false
	}
	var frame Frame
	if err := json.Unmarshal(encoded, &frame); err != nil {
		return Frame{}, false
	}
	return frame, true
}

// BeginTick starts one bounded decision frame before task execution.
func (r *Recorder) BeginTick(at time.Time, state WorldFrame, generation uint64, gates RuntimeGates, before TickState) {
	if r == nil || !r.Enabled() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current != nil {
		r.setFault(fmt.Errorf("runtime trace tick %d was not ended", r.current.Tick))
		return
	}
	r.tick++
	elapsed := at.Sub(r.started)
	if elapsed < 0 {
		r.setFault(fmt.Errorf("runtime trace monotonic time moved backwards"))
		return
	}
	r.current = &Frame{Tick: r.tick, ElapsedNS: elapsed.Nanoseconds(), SnapshotAtNS: at.UnixNano(), Generation: generation, Gates: gates, Before: before, World: state}
	if before.Step != "" && before.Step != r.lastStep {
		r.addCheckpoint(before)
		r.lastStep = before.Step
	}
}

// RecordDependency appends one ordered sanitized dependency observation to the current tick.
func (r *Recorder) RecordDependency(name string, args, result map[string]any, callErr error) {
	if r == nil || !r.Enabled() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil {
		return
	}
	r.sequence++
	record := DependencyCall{Sequence: r.sequence, Name: strings.TrimSpace(name), Args: SanitizeMap(args), Result: SanitizeMap(result)}
	if callErr != nil {
		record.Error = sanitizeText(callErr.Error())
	}
	r.current.Dependencies = append(r.current.Dependencies, record)
}

// RecordIntent appends a semantic input request or confirmed action. It never
// invokes the requested action itself.
func (r *Recorder) RecordIntent(name string, params map[string]any, outcome string) {
	if r == nil || !r.Enabled() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil {
		return
	}
	// A productive task tick owns at most one semantic input transaction. A
	// second intent would make ordering and replay safety ambiguous, so capture
	// fails closed instead of normalizing an already-invalid execution.
	if len(r.current.Intents) != 0 {
		r.setFault(fmt.Errorf("runtime trace tick %d produced more than one input intent", r.current.Tick))
		return
	}
	r.sequence++
	r.current.Intents = append(r.current.Intents, Intent{Sequence: r.sequence, Name: strings.TrimSpace(name), Params: SanitizeMap(params), Outcome: sanitizeText(outcome)})
}

// EndTick seals the frame after task execution and advances the bounded ring.
func (r *Recorder) EndTick(after TickState) {
	if r == nil || !r.Enabled() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil {
		r.setFault(fmt.Errorf("runtime trace ended a tick that was not started"))
		return
	}
	r.current.After = after
	if after.Step != "" && (after.Step != r.lastStep || after.Outcome != "running" || after.Reason != "") {
		r.addCheckpoint(after)
		r.lastStep = after.Step
	}
	r.frames = append(r.frames, *r.current)
	r.current = nil
	if overflow := len(r.frames) - r.config.MaximumFrames; overflow > 0 {
		r.truncated = true
		copy(r.frames, r.frames[overflow:])
		r.frames = r.frames[:r.config.MaximumFrames]
	}
}

func (r *Recorder) addCheckpoint(state TickState) {
	checkpoint := Checkpoint{Tick: r.tick, ElapsedNS: r.config.Now().Sub(r.started).Nanoseconds(), Step: state.Step, Outcome: state.Outcome, Reason: state.Reason}
	r.checks = append(r.checks, checkpoint)
	if overflow := len(r.checks) - defaultMaximumCheckpoints; overflow > 0 {
		copy(r.checks, r.checks[overflow:])
		r.checks = r.checks[:defaultMaximumCheckpoints]
	}
}

func (r *Recorder) setFault(err error) {
	if r.fault == nil {
		r.fault = err
	}
}

// Finalize atomically writes a terminal failure trace. Successful traces are
// discarded unless SaveSuccessful was explicitly configured.
func (r *Recorder) Finalize(terminal Terminal) (FinalizeResult, error) {
	if r == nil || !r.config.Enabled {
		return FinalizeResult{}, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return FinalizeResult{}, nil
	}
	r.closed = true
	if r.current != nil {
		r.current.After = TickState{Step: terminal.Step, Outcome: terminal.Outcome, Reason: terminal.Reason}
		r.frames = append(r.frames, *r.current)
		r.current = nil
	}
	if r.fault != nil {
		return FinalizeResult{}, r.fault
	}
	if terminal.Outcome == "success" && !r.config.SaveSuccessful {
		return FinalizeResult{}, nil
	}
	bundle := Bundle{SchemaVersion: SchemaVersion, Metadata: r.metadata, Contract: r.contract, Checkpoints: append([]Checkpoint(nil), r.checks...), Frames: append([]Frame(nil), r.frames...), FramesTruncated: r.truncated, Terminal: terminal}
	if err := bundle.Validate(); err != nil {
		return FinalizeResult{}, err
	}
	uncompressed, err := json.Marshal(bundle)
	if err != nil {
		return FinalizeResult{}, fmt.Errorf("measure runtime trace bundle: %w", err)
	}
	maximumUncompressedBytes := expandedByteLimit(r.config.MaximumBundleBytes)
	if int64(len(uncompressed)) > maximumUncompressedBytes {
		return FinalizeResult{}, fmt.Errorf("runtime trace uncompressed bundle exceeds %d bytes", maximumUncompressedBytes)
	}
	encoded, err := encodeBundle(bundle)
	if err != nil {
		return FinalizeResult{}, err
	}
	if int64(len(encoded)) > r.config.MaximumBundleBytes {
		return FinalizeResult{}, fmt.Errorf("runtime trace bundle exceeds %d bytes", r.config.MaximumBundleBytes)
	}
	if err := ensureTraceDirectory(r.config.Directory); err != nil {
		return FinalizeResult{}, err
	}
	filename, err := r.bundleFilename()
	if err != nil {
		return FinalizeResult{}, err
	}
	finalPath := filepath.Join(r.config.Directory, filename)
	temporary, err := os.CreateTemp(r.config.Directory, ".runtime-trace-*.tmp")
	if err != nil {
		return FinalizeResult{}, fmt.Errorf("create runtime trace staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(encoded); err != nil {
		_ = temporary.Close()
		return FinalizeResult{}, fmt.Errorf("write runtime trace staging file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return FinalizeResult{}, fmt.Errorf("sync runtime trace staging file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return FinalizeResult{}, fmt.Errorf("close runtime trace staging file: %w", err)
	}
	if err := r.rename(temporaryPath, finalPath); err != nil {
		return FinalizeResult{}, fmt.Errorf("publish runtime trace atomically: %w", err)
	}
	if err := enforceRetention(r.config.Directory, r.config.MaximumBundles, r.config.MaximumTotalBytes); err != nil {
		return FinalizeResult{}, err
	}
	return FinalizeResult{Saved: true, Filename: filename, Bytes: int64(len(encoded))}, nil
}

func encodeBundle(bundle Bundle) ([]byte, error) {
	var output bytes.Buffer
	zipper, err := gzip.NewWriterLevel(&output, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("create runtime trace compressor: %w", err)
	}
	encoder := json.NewEncoder(zipper)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(bundle); err != nil {
		_ = zipper.Close()
		return nil, fmt.Errorf("encode runtime trace bundle: %w", err)
	}
	if err := zipper.Close(); err != nil {
		return nil, fmt.Errorf("close runtime trace compressor: %w", err)
	}
	return output.Bytes(), nil
}

func ensureTraceDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("create runtime trace directory: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect runtime trace directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("runtime trace directory is not a regular directory")
	}
	return nil
}

func (r *Recorder) bundleFilename() (string, error) {
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("generate runtime trace filename: %w", err)
	}
	return fmt.Sprintf("%s-%s-%s%s", r.config.Label, r.metadata.CapturedAt.UTC().Format("20060102T150405Z"), hex.EncodeToString(suffix), BundleExtension), nil
}

type retainedTrace struct {
	name    string
	path    string
	size    int64
	modTime time.Time
}

func enforceRetention(directory string, maximumBundles int, maximumTotalBytes int64) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("scan runtime trace retention: %w", err)
	}
	traces := make([]retainedTrace, 0, len(entries))
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), BundleExtension) || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect runtime trace retention entry: %w", err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		trace := retainedTrace{name: entry.Name(), path: filepath.Join(directory, entry.Name()), size: info.Size(), modTime: info.ModTime()}
		traces = append(traces, trace)
		total += trace.size
	}
	sort.Slice(traces, func(i, j int) bool {
		if traces[i].modTime.Equal(traces[j].modTime) {
			return traces[i].name < traces[j].name
		}
		return traces[i].modTime.Before(traces[j].modTime)
	})
	for len(traces) > maximumBundles || total > maximumTotalBytes {
		oldest := traces[0]
		if err := os.Remove(oldest.path); err != nil {
			return fmt.Errorf("remove retained runtime trace %q: %w", oldest.name, err)
		}
		total -= oldest.size
		traces = traces[1:]
	}
	return nil
}

// ReadBundle decodes and validates one compressed runtime trace bundle.
func ReadBundle(path string, maximumBytes int64) (Bundle, error) {
	if maximumBytes <= 0 {
		maximumBytes = defaultMaximumBundleBytes
	}
	file, err := os.Open(path)
	if err != nil {
		return Bundle{}, fmt.Errorf("open runtime trace bundle: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Bundle{}, fmt.Errorf("inspect runtime trace bundle: %w", err)
	}
	if info.Size() > maximumBytes {
		return Bundle{}, fmt.Errorf("runtime trace compressed bundle exceeds %d bytes", maximumBytes)
	}
	zipper, err := gzip.NewReader(file)
	if err != nil {
		return Bundle{}, fmt.Errorf("open runtime trace compression: %w", err)
	}
	defer zipper.Close()
	maximumUncompressedBytes := expandedByteLimit(maximumBytes)
	decoded, err := io.ReadAll(io.LimitReader(zipper, maximumUncompressedBytes+1))
	if err != nil {
		return Bundle{}, fmt.Errorf("read runtime trace bundle: %w", err)
	}
	if int64(len(decoded)) > maximumUncompressedBytes {
		return Bundle{}, fmt.Errorf("runtime trace uncompressed payload exceeds %d bytes", maximumUncompressedBytes)
	}
	var bundle Bundle
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return Bundle{}, fmt.Errorf("decode runtime trace bundle: %w", err)
	}
	if err := bundle.Validate(); err != nil {
		return Bundle{}, err
	}
	return bundle, nil
}

func expandedByteLimit(compressedLimit int64) int64 {
	if compressedLimit > (1<<63-1)/maximumExpansionRatio {
		return 1<<63 - 1
	}
	return compressedLimit * maximumExpansionRatio
}
