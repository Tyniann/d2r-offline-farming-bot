package memory

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

type mockAccess struct {
	memory        map[uintptr][]byte
	moduleBase    uintptr
	readAtCalls   int
	failRemaining int
	readAtErr     error
	partialAt     uintptr
}

func newMockAccess() *mockAccess {
	return &mockAccess{memory: make(map[uintptr][]byte)}
}

func (m *mockAccess) setBytes(addr uintptr, data []byte) {
	m.memory[addr] = append([]byte(nil), data...)
}

func (m *mockAccess) ReadAt(addr uintptr, buf []byte) error {
	m.readAtCalls++
	if m.readAtErr != nil {
		return m.readAtErr
	}
	if m.failRemaining > 0 {
		m.failRemaining--
		return fmt.Errorf("read at %#x: %w", addr, process.ErrReadFailed)
	}
	if addr == m.partialAt {
		if len(buf) > 0 {
			buf[0] = 0xFF
		}
		return fmt.Errorf("read at %#x: %w", addr, process.ErrPartialRead)
	}

	data, ok := m.memory[addr]
	if !ok {
		return fmt.Errorf("read at %#x: %w", addr, process.ErrInvalidRead)
	}
	if len(data) < len(buf) {
		return fmt.Errorf("read at %#x: %w", addr, process.ErrPartialRead)
	}
	copy(buf, data[:len(buf)])
	return nil
}

func (m *mockAccess) ModuleBase() uintptr {
	return m.moduleBase
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestReader(access *mockAccess) *Reader {
	return newReaderWithRetry(testLogger(), retryConfig{
		attempts: 3,
		backoff:  time.Millisecond,
		sleep:    func(time.Duration) {},
	})
}

func TestReadUint32LittleEndian(t *testing.T) {
	access := newMockAccess()
	access.setBytes(0x1000, []byte{0x78, 0x56, 0x34, 0x12})

	r := newTestReader(access)
	r.Bind(access)

	got, err := r.ReadUint32(0x1000)
	if err != nil {
		t.Fatalf("ReadUint32() error = %v", err)
	}
	if got != 0x12345678 {
		t.Fatalf("ReadUint32() = %#x, want 0x12345678", got)
	}
}

func TestReadUint64LittleEndian(t *testing.T) {
	access := newMockAccess()
	access.setBytes(0x2000, []byte{0xEF, 0xBE, 0xAD, 0xDE, 0xBE, 0xBA, 0xFE, 0xCA})

	r := newTestReader(access)
	r.Bind(access)

	got, err := r.ReadUint64(0x2000)
	if err != nil {
		t.Fatalf("ReadUint64() error = %v", err)
	}
	if got != 0xCAFEBABEDEADBEEF {
		t.Fatalf("ReadUint64() = %#x, want 0xCAFEBABEDEADBEEF", got)
	}
}

func TestReadBytesReturnsCopy(t *testing.T) {
	access := newMockAccess()
	access.setBytes(0x3000, []byte{1, 2, 3})

	r := newTestReader(access)
	r.Bind(access)

	got, err := r.ReadBytes(0x3000, 3)
	if err != nil {
		t.Fatalf("ReadBytes() error = %v", err)
	}
	got[0] = 99
	if access.memory[0x3000][0] != 1 {
		t.Fatal("ReadBytes should return a copy, not alias mock memory")
	}
}

func TestReadBytesInvalidAddress(t *testing.T) {
	access := newMockAccess()
	r := newTestReader(access)
	r.Bind(access)

	cases := []struct {
		name string
		addr uintptr
		size int
	}{
		{"zero address", 0, 4},
		{"zero size", 0x1000, 0},
		{"negative size", 0x1000, -1},
		{"too large", 0x1000, maxReadSize + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			access.readAtCalls = 0
			_, err := r.ReadBytes(tc.addr, tc.size)
			if !errors.Is(err, ErrInvalidAddress) {
				t.Fatalf("ReadBytes() error = %v, want ErrInvalidAddress", err)
			}
			if access.readAtCalls != 0 {
				t.Fatalf("readAtCalls = %d, want 0 (no process read)", access.readAtCalls)
			}
		})
	}
}

func TestReaderNotBound(t *testing.T) {
	r := newTestReader(nil)

	_, err := r.ReadBytes(0x1000, 4)
	if !errors.Is(err, ErrNotBound) {
		t.Fatalf("ReadBytes() error = %v, want ErrNotBound", err)
	}

	_, err = r.ResolvePointerChain(0x1000, 0x10)
	if !errors.Is(err, ErrNotBound) {
		t.Fatalf("ResolvePointerChain() error = %v, want ErrNotBound", err)
	}
}

func TestReadRetriesTransientFailure(t *testing.T) {
	access := newMockAccess()
	access.failRemaining = 2
	access.setBytes(0x4000, []byte{0x01, 0x00, 0x00, 0x00})

	r := newTestReader(access)
	r.Bind(access)

	got, err := r.ReadUint32(0x4000)
	if err != nil {
		t.Fatalf("ReadUint32() error = %v", err)
	}
	if got != 1 {
		t.Fatalf("ReadUint32() = %d, want 1", got)
	}
	if access.readAtCalls != 3 {
		t.Fatalf("readAtCalls = %d, want 3", access.readAtCalls)
	}
}

func TestReadExhaustsRetries(t *testing.T) {
	access := newMockAccess()
	access.failRemaining = 5

	r := newTestReader(access)
	r.Bind(access)

	_, err := r.ReadBytes(0x5000, 4)
	if err == nil {
		t.Fatal("ReadBytes() expected error after retries exhausted")
	}
	if !errors.Is(err, process.ErrReadFailed) {
		t.Fatalf("ReadBytes() error = %v, want ErrReadFailed", err)
	}
	if access.readAtCalls != 3 {
		t.Fatalf("readAtCalls = %d, want 3", access.readAtCalls)
	}
}

func TestReadDoesNotRetryStructuralErrors(t *testing.T) {
	access := newMockAccess()
	r := newTestReader(access)
	r.Bind(access)

	cases := []struct {
		name string
		err  error
	}{
		{"not attached", process.ErrNotAttached},
		{"partial read", process.ErrPartialRead},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			access.readAtErr = fmt.Errorf("wrapped: %w", tc.err)
			access.readAtCalls = 0

			_, err := r.ReadBytes(0x6000, 4)
			if !errors.Is(err, tc.err) {
				t.Fatalf("ReadBytes() error = %v, want %v", err, tc.err)
			}
			if access.readAtCalls != 1 {
				t.Fatalf("readAtCalls = %d, want 1 (no retry)", access.readAtCalls)
			}
			access.readAtErr = nil
		})
	}
}

func TestResolvePointerChainMultiLevel(t *testing.T) {
	access := newMockAccess()
	// Chain: A000 -> (B000+0x10)=B010 -> (C000+0x20)=C020 -> (D000+0x30)=D030
	access.setBytes(0xA000, uint64ToLE(0xB000))
	access.setBytes(0xB010, uint64ToLE(0xC000))
	access.setBytes(0xC020, uint64ToLE(0xD000))

	r := newTestReader(access)
	r.Bind(access)

	got, err := r.ResolvePointerChain(0xA000, 0x10, 0x20, 0x30)
	if err != nil {
		t.Fatalf("ResolvePointerChain() error = %v", err)
	}
	want := uintptr(0xD030)
	if got != want {
		t.Fatalf("ResolvePointerChain() = %#x, want %#x", got, want)
	}
}

func TestResolvePointerChainNoOffsets(t *testing.T) {
	access := newMockAccess()
	r := newTestReader(access)
	r.Bind(access)

	got, err := r.ResolvePointerChain(0xBEEF)
	if err != nil {
		t.Fatalf("ResolvePointerChain() error = %v", err)
	}
	if got != 0xBEEF {
		t.Fatalf("ResolvePointerChain() = %#x, want 0xBEEF", got)
	}
}

func TestResolvePointerChainNullPointer(t *testing.T) {
	access := newMockAccess()
	access.setBytes(0xE000, uint64ToLE(0))

	r := newTestReader(access)
	r.Bind(access)

	_, err := r.ResolvePointerChain(0xE000, 0x10)
	if !errors.Is(err, ErrInvalidPointer) {
		t.Fatalf("ResolvePointerChain() error = %v, want ErrInvalidPointer", err)
	}
}

func TestResolvePointerChainUnreadableAddress(t *testing.T) {
	access := newMockAccess()
	r := newTestReader(access)
	r.Bind(access)

	_, err := r.ResolvePointerChain(0xF000, 0x10)
	if errors.Is(err, ErrInvalidPointer) {
		t.Fatalf("unreadable address should not map to ErrInvalidPointer, got %v", err)
	}
	if !errors.Is(err, process.ErrInvalidRead) {
		t.Fatalf("ResolvePointerChain() error = %v, want process read error", err)
	}
}

func uint64ToLE(v uint64) []byte {
	return []byte{
		byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24),
		byte(v >> 32), byte(v >> 40), byte(v >> 48), byte(v >> 56),
	}
}
