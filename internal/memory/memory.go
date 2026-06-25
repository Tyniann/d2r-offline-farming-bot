package memory

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

const maxReadSize = 64 * 1024

var (
	// ErrNotBound indicates the reader has no [ProcessAccess] bound.
	ErrNotBound = errors.New("memory reader not bound to process")

	// ErrInvalidAddress indicates address 0 or an out-of-bounds read size.
	ErrInvalidAddress = errors.New("invalid memory address or size")

	// ErrInvalidPointer indicates a null pointer was encountered while resolving a chain.
	ErrInvalidPointer = errors.New("null pointer in chain")
)

// ProcessAccess is the narrow interface [Reader] uses to read from the game process.
// [process.Service] satisfies this interface.
type ProcessAccess interface {
	ReadAt(addr uintptr, buf []byte) error
	ModuleBase() uintptr
}

type retryConfig struct {
	attempts int
	backoff  time.Duration
	sleep    func(time.Duration)
}

// Reader reads raw bytes from an attached process and decodes primitive types.
type Reader struct {
	log    *slog.Logger
	access ProcessAccess
	retry  retryConfig
}

// NewReader creates a memory reader with the default retry policy (3 attempts, 2 ms backoff).
func NewReader(log *slog.Logger) *Reader {
	return newReaderWithRetry(log, retryConfig{
		attempts: 3,
		backoff:  2 * time.Millisecond,
		sleep:    time.Sleep,
	})
}

func newReaderWithRetry(log *slog.Logger, retry retryConfig) *Reader {
	return &Reader{
		log:   log.With("component", "memory"),
		retry: retry,
	}
}

// Ready reports whether the reader is initialized (not whether a process is bound).
func (r *Reader) Ready() bool {
	r.log.Debug("memory reader ready")
	return true
}

// Bind wires the reader to a [ProcessAccess] implementation, typically once at startup.
func (r *Reader) Bind(access ProcessAccess) {
	r.access = access
}

// ReadBytes reads size bytes at addr and returns a copy of the data.
func (r *Reader) ReadBytes(addr uintptr, size int) ([]byte, error) {
	if r.access == nil {
		return nil, fmt.Errorf("read %d bytes at %#x: %w", size, addr, ErrNotBound)
	}
	if addr == 0 || size <= 0 || size > maxReadSize {
		return nil, fmt.Errorf("read %d bytes at %#x: %w", size, addr, ErrInvalidAddress)
	}

	buf := make([]byte, size)
	if err := r.readWithRetry(addr, buf); err != nil {
		return nil, fmt.Errorf("read %d bytes at %#x: %w", size, addr, err)
	}

	out := make([]byte, size)
	copy(out, buf)
	return out, nil
}

// ReadUint32 reads a little-endian uint32 at addr.
func (r *Reader) ReadUint32(addr uintptr) (uint32, error) {
	buf, err := r.ReadBytes(addr, 4)
	if err != nil {
		return 0, fmt.Errorf("read uint32 at %#x: %w", addr, err)
	}
	return binary.LittleEndian.Uint32(buf), nil
}

// ReadUint64 reads a little-endian uint64 at addr.
func (r *Reader) ReadUint64(addr uintptr) (uint64, error) {
	buf, err := r.ReadBytes(addr, 8)
	if err != nil {
		return 0, fmt.Errorf("read uint64 at %#x: %w", addr, err)
	}
	return binary.LittleEndian.Uint64(buf), nil
}

// ResolvePointerChain walks a pointer chain starting at base.
// For each offset, a uint64 pointer is read at the current address, checked for zero,
// then the offset is added. base is typically an absolute address such as moduleBase + staticOffset.
// The returned address is the final target for a subsequent field read, not the field value itself.
// An empty offset list returns base unchanged.
func (r *Reader) ResolvePointerChain(base uintptr, offsets ...uintptr) (uintptr, error) {
	if r.access == nil {
		return 0, fmt.Errorf("resolve pointer chain from %#x: %w", base, ErrNotBound)
	}
	if len(offsets) == 0 {
		return base, nil
	}

	addr := base
	for i, off := range offsets {
		ptr, err := r.ReadUint64(addr)
		if err != nil {
			return 0, fmt.Errorf("resolve pointer chain step %d at %#x: %w", i, addr, err)
		}
		if ptr == 0 {
			return 0, fmt.Errorf("resolve pointer chain step %d at %#x: %w", i, addr, ErrInvalidPointer)
		}
		addr = uintptr(ptr) + off
	}
	return addr, nil
}

func (r *Reader) readWithRetry(addr uintptr, buf []byte) error {
	var lastErr error
	for attempt := 0; attempt < r.retry.attempts; attempt++ {
		if attempt > 0 {
			r.log.Debug("retrying memory read",
				"addr", fmt.Sprintf("0x%X", addr),
				"attempt", attempt+1,
			)
			r.retry.sleep(r.retry.backoff)
		}

		lastErr = r.access.ReadAt(addr, buf)
		if lastErr == nil {
			return nil
		}
		if !process.IsReadRetryable(lastErr) {
			return lastErr
		}
	}
	return lastErr
}
