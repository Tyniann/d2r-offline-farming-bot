package process

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// State describes the lifecycle state of the process service.
type State string

const (
	// StateDetached means no process handle is held.
	StateDetached State = "detached"
	// StateAttached means a live process handle is held.
	StateAttached State = "attached"
	// StateLost means the previously attached process has exited.
	StateLost State = "lost"
)

// Status is a read-only snapshot of the process service state.
type Status struct {
	State      State
	PID        uint32
	Process    string
	ModuleBase uintptr
	ModuleSize uint32
	LastError  string
}

// Service finds and manages the D2R game process.
type Service struct {
	log         *slog.Logger
	processName string
	api         processAPI

	mu     sync.Mutex
	state  State
	status Status
	handle nativeHandle
}

// New creates a process service targeting the given executable name.
func New(log *slog.Logger, processName string) *Service {
	return newWithAPI(log, processName, defaultAPI())
}

func newWithAPI(log *slog.Logger, processName string, api processAPI) *Service {
	return &Service{
		log:         log.With("component", "process"),
		processName: processName,
		api:         api,
		state:       StateDetached,
		status: Status{
			State:   StateDetached,
			Process: processName,
		},
	}
}

// Ready reports whether the service is initialized (not whether a process is attached).
func (s *Service) Ready() bool {
	s.log.Debug("process service ready", "target", s.processName)
	return true
}

// Attach finds the target process, opens a read handle, and resolves the module base address.
// Allowed transitions: detached -> attached, lost -> attached.
func (s *Service) Attach(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == StateAttached {
		return fmt.Errorf("%w", ErrAlreadyAttached)
	}

	info, err := s.api.FindProcessByName(s.processName)
	if err != nil {
		s.setLastErrorLocked(err.Error())
		return err
	}

	handle, err := s.api.OpenReadHandle(info.PID)
	if err != nil {
		s.setLastErrorLocked(err.Error())
		return err
	}

	moduleBase, moduleSize, err := s.api.ModuleImage(info.PID, s.processName)
	if err != nil {
		_ = s.api.Close(handle)
		s.setLastErrorLocked(err.Error())
		return err
	}

	s.handle = handle
	s.state = StateAttached
	s.status = Status{
		State:      StateAttached,
		PID:        info.PID,
		Process:    s.processName,
		ModuleBase: moduleBase,
		ModuleSize: moduleSize,
	}
	s.log.Debug("process attached",
		"pid", info.PID,
		"process", s.processName,
		"module_base", fmt.Sprintf("0x%X", moduleBase),
	)
	return nil
}

// Poll actively checks whether the attached process is still alive.
// It mutates state when the process has exited (attached -> lost).
func (s *Service) Poll() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateAttached {
		return s.status
	}

	if s.api.IsAlive(s.handle) {
		return s.status
	}

	_ = s.api.Close(s.handle)
	s.handle = 0
	s.state = StateLost
	s.status = Status{
		State:      StateLost,
		PID:        s.status.PID,
		Process:    s.processName,
		ModuleBase: s.status.ModuleBase,
	}
	s.log.Debug("process lost", "pid", s.status.PID, "process", s.processName)
	return s.status
}

// Status returns a read-only snapshot without performing lifecycle checks.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// ModuleBase returns the base address of the attached module, or zero if not attached.
func (s *Service) ModuleBase() uintptr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status.ModuleBase
}

// ModuleSize returns the image size of the attached module, or zero if not attached.
func (s *Service) ModuleSize() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status.ModuleSize
}

// ReadAt reads raw bytes from the attached process at the given virtual address.
// The service mutex is held for the entire operation to avoid use-after-close of the handle.
// Retries are the responsibility of [memory.Reader]; this method performs a single OS read.
func (s *Service) ReadAt(addr uintptr, buf []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateAttached {
		return fmt.Errorf("read at %#x: not attached (%s): %w", addr, s.state, ErrNotAttached)
	}
	if s.handle == 0 {
		return fmt.Errorf("read at %#x: %w", addr, ErrNotAttached)
	}
	if addr == 0 {
		return fmt.Errorf("read at address 0: %w", ErrInvalidRead)
	}
	if len(buf) == 0 {
		return fmt.Errorf("read with empty buffer: %w", ErrInvalidRead)
	}

	return s.api.ReadMemory(s.handle, addr, buf)
}

// Detach closes any open handle and resets state to detached. It is idempotent.
func (s *Service) Detach() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.handle != 0 {
		if err := s.api.Close(s.handle); err != nil {
			return fmt.Errorf("close process handle: %w", err)
		}
		s.handle = 0
	}

	if s.state != StateDetached {
		s.log.Debug("process detached", "process", s.processName)
	}

	s.state = StateDetached
	s.status = Status{
		State:   StateDetached,
		Process: s.processName,
	}
	return nil
}

func (s *Service) setLastErrorLocked(msg string) {
	s.status.LastError = msg
}
