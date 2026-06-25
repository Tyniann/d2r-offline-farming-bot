package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockAPI struct {
	processes   map[string][]ProcessInfo
	handles     map[uint32]nativeHandle
	moduleBases map[uint32]uintptr
	moduleSizes map[uint32]uint32
	alive       map[nativeHandle]bool
	nextHandle  atomic.Uint64
	openErr     error
	moduleErr   error
	findErr     error
	closeCount  atomic.Int32
	findCalls   atomic.Int32
	moduleCalls atomic.Int32
	memory      map[uintptr][]byte
	readMemoryFn func(handle nativeHandle, addr uintptr, buf []byte) error
	readCalls   atomic.Int32
	lastRead    readMemoryCall
	mu          sync.Mutex
}

type readMemoryCall struct {
	handle nativeHandle
	addr   uintptr
	length int
}

func newMockAPI() *mockAPI {
	return &mockAPI{
		processes:   make(map[string][]ProcessInfo),
		handles:     make(map[uint32]nativeHandle),
		moduleBases: make(map[uint32]uintptr),
		moduleSizes: make(map[uint32]uint32),
		alive:       make(map[nativeHandle]bool),
		memory:      make(map[uintptr][]byte),
	}
}

func (m *mockAPI) addProcess(name string, pid uint32, moduleBase uintptr) nativeHandle {
	m.mu.Lock()
	defer m.mu.Unlock()

	info := ProcessInfo{PID: pid, Name: name}
	m.processes[strings.ToLower(name)] = append(m.processes[strings.ToLower(name)], info)
	m.moduleBases[pid] = moduleBase
	m.moduleSizes[pid] = 32 * 1024 * 1024

	handle := nativeHandle(m.nextHandle.Add(1))
	m.handles[pid] = handle
	m.alive[handle] = true
	return handle
}

func (m *mockAPI) FindProcessByName(name string) (ProcessInfo, error) {
	m.findCalls.Add(1)
	if m.findErr != nil {
		return ProcessInfo{}, m.findErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	matches := m.processes[strings.ToLower(name)]
	switch len(matches) {
	case 0:
		return ProcessInfo{}, fmt.Errorf("find process %s: %w", name, ErrNotFound)
	case 1:
		return matches[0], nil
	default:
		return ProcessInfo{}, fmt.Errorf("find process %s: %w", name, ErrMultipleInstances)
	}
}

func (m *mockAPI) OpenReadHandle(pid uint32) (nativeHandle, error) {
	if m.openErr != nil {
		return 0, m.openErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	handle, ok := m.handles[pid]
	if !ok {
		return 0, fmt.Errorf("open process pid=%s: %w", strconv.FormatUint(uint64(pid), 10), ErrAccessDenied)
	}
	return handle, nil
}

func (m *mockAPI) ModuleImage(pid uint32, moduleName string) (uintptr, uint32, error) {
	m.moduleCalls.Add(1)
	if m.moduleErr != nil {
		return 0, 0, m.moduleErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	base, ok := m.moduleBases[pid]
	if !ok {
		return 0, 0, fmt.Errorf("module %s: %w", moduleName, ErrModuleNotFound)
	}
	size := m.moduleSizes[pid]
	if size == 0 {
		size = 32 * 1024 * 1024
	}
	return base, size, nil
}

func (m *mockAPI) IsAlive(handle nativeHandle) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.alive[handle]
}

func (m *mockAPI) setAlive(handle nativeHandle, alive bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alive[handle] = alive
}

func (m *mockAPI) Close(handle nativeHandle) error {
	m.closeCount.Add(1)
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.alive, handle)
	return nil
}

func (m *mockAPI) setMemory(addr uintptr, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memory[addr] = append([]byte(nil), data...)
}

func (m *mockAPI) ReadMemory(handle nativeHandle, addr uintptr, buf []byte) error {
	m.readCalls.Add(1)
	if m.readMemoryFn != nil {
		return m.readMemoryFn(handle, addr, buf)
	}

	m.mu.Lock()
	m.lastRead = readMemoryCall{handle: handle, addr: addr, length: len(buf)}

	data, ok := m.memory[addr]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("read memory at %#x: %w", addr, ErrReadFailed)
	}
	if len(data) < len(buf) {
		copy(buf, data)
		m.mu.Unlock()
		return fmt.Errorf("read memory at %#x: read %d of %d bytes: %w",
			addr, len(data), len(buf), ErrPartialRead)
	}
	copy(buf, data[:len(buf)])
	m.mu.Unlock()
	return nil
}

func (m *mockAPI) lastReadCall() readMemoryCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastRead
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestAttachSuccess(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 4242, 0x140000000)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	if err := svc.Attach(context.Background()); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}

	st := svc.Status()
	if st.State != StateAttached {
		t.Fatalf("State = %q, want attached", st.State)
	}
	if st.PID != 4242 {
		t.Fatalf("PID = %d, want 4242", st.PID)
	}
	if st.ModuleBase != 0x140000000 {
		t.Fatalf("ModuleBase = %#x, want 0x140000000", st.ModuleBase)
	}
}

func TestAttachNotFound(t *testing.T) {
	svc := newWithAPI(testLogger(), "D2R.exe", newMockAPI())
	err := svc.Attach(context.Background())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Attach() error = %v, want ErrNotFound", err)
	}
	if svc.Status().State != StateDetached {
		t.Fatalf("State = %q, want detached", svc.Status().State)
	}
}

func TestAttachMultipleInstances(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 1, 0x1000)
	api.addProcess("D2R.exe", 2, 0x2000)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	err := svc.Attach(context.Background())
	if !errors.Is(err, ErrMultipleInstances) {
		t.Fatalf("Attach() error = %v, want ErrMultipleInstances", err)
	}
}

func TestAttachAlreadyAttached(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 99, 0x1000)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	if err := svc.Attach(context.Background()); err != nil {
		t.Fatalf("first Attach() error = %v", err)
	}

	err := svc.Attach(context.Background())
	if !errors.Is(err, ErrAlreadyAttached) {
		t.Fatalf("second Attach() error = %v, want ErrAlreadyAttached", err)
	}
}

func TestAttachOpenProcessFailureDoesNotClose(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 77, 0x1000)
	api.openErr = fmt.Errorf("open process pid=77: %w", ErrAccessDenied)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	err := svc.Attach(context.Background())
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("Attach() error = %v, want ErrAccessDenied", err)
	}
	if api.closeCount.Load() != 0 {
		t.Fatalf("Close calls = %d, want 0", api.closeCount.Load())
	}
}

func TestAttachModuleFailureClosesHandle(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 55, 0)
	api.moduleErr = fmt.Errorf("module D2R.exe: %w", ErrModuleNotFound)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	err := svc.Attach(context.Background())
	if !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("Attach() error = %v, want ErrModuleNotFound", err)
	}
	if api.closeCount.Load() != 1 {
		t.Fatalf("Close calls = %d, want 1", api.closeCount.Load())
	}
}

func TestPollDetectsProcessLoss(t *testing.T) {
	api := newMockAPI()
	handle := api.addProcess("D2R.exe", 10, 0x2000)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	if err := svc.Attach(context.Background()); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}

	api.setAlive(handle, false)
	st := svc.Poll()
	if st.State != StateLost {
		t.Fatalf("Poll() State = %q, want lost", st.State)
	}
	if api.closeCount.Load() != 1 {
		t.Fatalf("Close calls = %d, want 1", api.closeCount.Load())
	}
}

func TestPollDetachedAndLostAreNoOps(t *testing.T) {
	svc := newWithAPI(testLogger(), "D2R.exe", newMockAPI())

	st := svc.Poll()
	if st.State != StateDetached {
		t.Fatalf("Poll() detached State = %q, want detached", st.State)
	}

	api := newMockAPI()
	handle := api.addProcess("D2R.exe", 10, 0x2000)
	svc = newWithAPI(testLogger(), "D2R.exe", api)
	_ = svc.Attach(context.Background())
	api.setAlive(handle, false)
	_ = svc.Poll()

	before := api.closeCount.Load()
	st = svc.Poll()
	if st.State != StateLost {
		t.Fatalf("Poll() lost State = %q, want lost", st.State)
	}
	if api.closeCount.Load() != before {
		t.Fatalf("Close calls changed from %d to %d on lost Poll", before, api.closeCount.Load())
	}
}

func TestAttachAfterLost(t *testing.T) {
	api := newMockAPI()
	handle := api.addProcess("D2R.exe", 10, 0x3000)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	if err := svc.Attach(context.Background()); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}

	api.setAlive(handle, false)
	_ = svc.Poll()

	if err := svc.Attach(context.Background()); err != nil {
		t.Fatalf("re-Attach() error = %v", err)
	}
	if svc.Status().State != StateAttached {
		t.Fatalf("State = %q, want attached", svc.Status().State)
	}
}

func TestDetachIdempotent(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 10, 0x3000)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	_ = svc.Attach(context.Background())

	if err := svc.Detach(); err != nil {
		t.Fatalf("first Detach() error = %v", err)
	}
	if err := svc.Detach(); err != nil {
		t.Fatalf("second Detach() error = %v", err)
	}
	if api.closeCount.Load() != 1 {
		t.Fatalf("Close calls = %d, want 1", api.closeCount.Load())
	}
	if svc.Status().State != StateDetached {
		t.Fatalf("State = %q, want detached", svc.Status().State)
	}
}

func TestStatusIsReadOnly(t *testing.T) {
	api := newMockAPI()
	handle := api.addProcess("D2R.exe", 10, 0x3000)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	_ = svc.Attach(context.Background())

	api.setAlive(handle, false)
	st := svc.Status()
	if st.State != StateAttached {
		t.Fatalf("Status() State = %q, want attached before Poll", st.State)
	}
}

func TestCaseInsensitiveProcessName(t *testing.T) {
	api := newMockAPI()
	api.addProcess("d2r.exe", 42, 0x1000)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	if err := svc.Attach(context.Background()); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
}

func TestCaseInsensitiveModuleName(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 42, 0xABCDEF)

	svc := newWithAPI(testLogger(), "d2r.exe", api)
	if err := svc.Attach(context.Background()); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	if svc.ModuleBase() != 0xABCDEF {
		t.Fatalf("ModuleBase = %#x, want 0xABCDEF", svc.ModuleBase())
	}
}

func TestIsRetryableAndFatal(t *testing.T) {
	if !IsRetryable(ErrNotFound) {
		t.Fatal("ErrNotFound should be retryable")
	}
	if IsFatal(ErrNotFound) {
		t.Fatal("ErrNotFound should not be fatal")
	}
	for _, err := range []error{ErrMultipleInstances, ErrAccessDenied, ErrModuleNotFound} {
		if !IsFatal(err) {
			t.Fatalf("%v should be fatal", err)
		}
		if IsRetryable(err) {
			t.Fatalf("%v should not be retryable", err)
		}
	}
}

func TestConcurrentPollAndDetach(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 10, 0x3000)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	if err := svc.Attach(context.Background()); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.Poll()
		}()
	}
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.Detach()
		}()
	}
	wg.Wait()
}

func TestReadAtDetachedAndLost(t *testing.T) {
	svc := newWithAPI(testLogger(), "D2R.exe", newMockAPI())
	buf := make([]byte, 4)

	err := svc.ReadAt(0x1000, buf)
	if !errors.Is(err, ErrNotAttached) {
		t.Fatalf("ReadAt() detached error = %v, want ErrNotAttached", err)
	}

	api := newMockAPI()
	handle := api.addProcess("D2R.exe", 10, 0x2000)
	svc = newWithAPI(testLogger(), "D2R.exe", api)
	_ = svc.Attach(context.Background())
	api.setAlive(handle, false)
	_ = svc.Poll()

	err = svc.ReadAt(0x1000, buf)
	if !errors.Is(err, ErrNotAttached) {
		t.Fatalf("ReadAt() lost error = %v, want ErrNotAttached", err)
	}
}

func TestReadAtEmptyBuffer(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 10, 0x2000)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	_ = svc.Attach(context.Background())

	err := svc.ReadAt(0x1000, nil)
	if !errors.Is(err, ErrInvalidRead) {
		t.Fatalf("ReadAt() error = %v, want ErrInvalidRead", err)
	}
	if api.readCalls.Load() != 0 {
		t.Fatalf("ReadMemory calls = %d, want 0", api.readCalls.Load())
	}
}

func TestReadAtZeroAddress(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 10, 0x2000)

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	_ = svc.Attach(context.Background())

	err := svc.ReadAt(0, make([]byte, 4))
	if !errors.Is(err, ErrInvalidRead) {
		t.Fatalf("ReadAt() error = %v, want ErrInvalidRead", err)
	}
}

func TestReadAtAttachedSuccess(t *testing.T) {
	api := newMockAPI()
	handle := api.addProcess("D2R.exe", 10, 0x2000)
	api.setMemory(0x5000, []byte{0xDE, 0xAD, 0xBE, 0xEF})

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	_ = svc.Attach(context.Background())

	buf := make([]byte, 4)
	if err := svc.ReadAt(0x5000, buf); err != nil {
		t.Fatalf("ReadAt() error = %v", err)
	}
	if api.readCalls.Load() != 1 {
		t.Fatalf("ReadMemory calls = %d, want 1", api.readCalls.Load())
	}
	call := api.lastReadCall()
	if call.handle != handle {
		t.Fatalf("ReadMemory handle = %#x, want %#x", call.handle, handle)
	}
	if call.addr != 0x5000 {
		t.Fatalf("ReadMemory addr = %#x, want 0x5000", call.addr)
	}
	if call.length != 4 {
		t.Fatalf("ReadMemory buflen = %d, want 4", call.length)
	}
	want := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	for i := range want {
		if buf[i] != want[i] {
			t.Fatalf("buf[%d] = %#x, want %#x", i, buf[i], want[i])
		}
	}
}

func TestReadAtPartialRead(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 10, 0x2000)
	api.setMemory(0x6000, []byte{0x01})

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	_ = svc.Attach(context.Background())

	err := svc.ReadAt(0x6000, make([]byte, 4))
	if !errors.Is(err, ErrPartialRead) {
		t.Fatalf("ReadAt() error = %v, want ErrPartialRead", err)
	}
}

func TestReadAtInvalidReadFromAPI(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 10, 0x2000)
	api.readMemoryFn = func(_ nativeHandle, addr uintptr, _ []byte) error {
		return fmt.Errorf("read memory at %#x: %w", addr, ErrInvalidRead)
	}

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	_ = svc.Attach(context.Background())

	err := svc.ReadAt(0x8000, make([]byte, 8))
	if !errors.Is(err, ErrInvalidRead) {
		t.Fatalf("ReadAt() error = %v, want ErrInvalidRead", err)
	}
}

func TestReadAtLostStateInError(t *testing.T) {
	api := newMockAPI()
	handle := api.addProcess("D2R.exe", 10, 0x2000)
	svc := newWithAPI(testLogger(), "D2R.exe", api)
	_ = svc.Attach(context.Background())
	api.setAlive(handle, false)
	_ = svc.Poll()

	err := svc.ReadAt(0x1000, make([]byte, 4))
	if !errors.Is(err, ErrNotAttached) {
		t.Fatalf("ReadAt() error = %v, want ErrNotAttached", err)
	}
	if !strings.Contains(err.Error(), "lost") {
		t.Fatalf("ReadAt() error = %q, want state lost in message", err.Error())
	}
}

func TestReadAtBlocksPollWhileReading(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 10, 0x2000)

	readStarted := make(chan struct{})
	releaseRead := make(chan struct{})
	api.readMemoryFn = func(_ nativeHandle, _ uintptr, _ []byte) error {
		close(readStarted)
		<-releaseRead
		return nil
	}

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	_ = svc.Attach(context.Background())

	readDone := make(chan error, 1)
	go func() {
		readDone <- svc.ReadAt(0x9000, make([]byte, 4))
	}()

	<-readStarted

	pollDone := make(chan struct{})
	go func() {
		_ = svc.Poll()
		close(pollDone)
	}()

	select {
	case <-pollDone:
		t.Fatal("Poll() should block while ReadAt holds the service mutex")
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseRead)
	if err := <-readDone; err != nil {
		t.Fatalf("ReadAt() error = %v", err)
	}

	select {
	case <-pollDone:
	case <-time.After(time.Second):
		t.Fatal("Poll() did not complete after ReadAt finished")
	}
}

func TestIsReadRetryable(t *testing.T) {
	if IsReadRetryable(nil) {
		t.Fatal("nil should not be retryable")
	}
	if !IsReadRetryable(fmt.Errorf("wrapped: %w", ErrReadFailed)) {
		t.Fatal("ErrReadFailed should be retryable")
	}
	for _, err := range []error{ErrNotAttached, ErrPartialRead, ErrInvalidRead} {
		if IsReadRetryable(err) {
			t.Fatalf("%v should not be retryable", err)
		}
	}
}

func TestConcurrentReadAtAndDetach(t *testing.T) {
	api := newMockAPI()
	api.addProcess("D2R.exe", 10, 0x3000)
	api.setMemory(0x7000, []byte{1, 2, 3, 4})

	svc := newWithAPI(testLogger(), "D2R.exe", api)
	if err := svc.Attach(context.Background()); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 4)
			_ = svc.ReadAt(0x7000, buf)
		}()
	}
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.Detach()
		}()
	}
	wg.Wait()
}
