package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

// QueueReason is a stable machine-readable queue validation or budget reason.
type QueueReason string

const (
	// QueueReasonEmpty rejects a queue without entries.
	QueueReasonEmpty QueueReason = "queue_empty"
	// QueueReasonDuplicateRun rejects a repeated run before process attach or input.
	QueueReasonDuplicateRun QueueReason = "queue_duplicate_run"
	// QueueReasonEntryUnavailable rejects an unknown or unavailable entry.
	QueueReasonEntryUnavailable QueueReason = "queue_entry_unavailable"
	// QueueReasonContextMismatch rejects a queue for another confirmed selection.
	QueueReasonContextMismatch QueueReason = "queue_context_mismatch"
	// QueueReasonLocked rejects mutation while a queue is active.
	QueueReasonLocked QueueReason = "queue_locked"
	// QueueReasonRunBudgetExhausted completes before another run would exceed `max_runs`.
	QueueReasonRunBudgetExhausted QueueReason = "run_budget_exhausted"
	// QueueReasonDurationBudgetExhausted completes before another run would exceed `max_duration_ms`.
	QueueReasonDurationBudgetExhausted QueueReason = "duration_budget_exhausted"
)

// FarmQueueBudgets are immutable hard limits copied from YAML for one runtime queue.
type FarmQueueBudgets struct {
	MaxRuns                int
	MaxDuration            time.Duration
	MaxConsecutiveFailures int
	MaxTotalRestarts       int
}

// FarmQueueValidationRequest is the complete proposed runtime-only queue.
type FarmQueueValidationRequest struct {
	RunIDs          []string
	Character       string
	Difficulty      string
	CatalogRevision uint64
}

// FarmQueueValidationContext is the authoritative confirmed selection and catalog revision.
type FarmQueueValidationContext struct {
	Character       string
	Difficulty      string
	CatalogRevision uint64
}

// FarmQueuePlan is an immutable preflight result accepted by [SessionSupervisor].
type FarmQueuePlan struct {
	RunIDs          []string
	Character       string
	Difficulty      string
	CatalogRevision uint64
	Budgets         FarmQueueBudgets
}

// QueueValidationError identifies the rejected entry without translating its underlying reasons.
type QueueValidationError struct {
	Code       QueueReason
	EntryIndex int
	FirstIndex int
	RunID      string
	Reasons    []tasks.RunReason
}

// Error implements error.
func (e *QueueValidationError) Error() string {
	if e == nil {
		return ""
	}
	if e.RunID == "" {
		return string(e.Code)
	}
	if e.Code == QueueReasonDuplicateRun {
		return fmt.Sprintf("%s: queue[%d]=%q duplicates queue[%d]", e.Code, e.EntryIndex, e.RunID, e.FirstIndex)
	}
	return fmt.Sprintf("%s: queue[%d]=%q: %s", e.Code, e.EntryIndex, e.RunID, joinRunReasons(e.Reasons))
}

// ValidateFarmQueue resolves every entry against one confirmed context and one catalog revision.
// Duplicate IDs fail before availability resolution; no worker, process attach, or input is created here.
func ValidateFarmQueue(cfg *config.Config, request FarmQueueValidationRequest, current FarmQueueValidationContext) (FarmQueuePlan, error) {
	if cfg == nil {
		return FarmQueuePlan{}, fmt.Errorf("farm queue validation requires config")
	}
	if len(request.RunIDs) == 0 {
		return FarmQueuePlan{}, &QueueValidationError{Code: QueueReasonEmpty, EntryIndex: -1}
	}
	queue, err := validateUniqueQueueRunIDs(request.RunIDs)
	if err != nil {
		return FarmQueuePlan{}, err
	}
	if request.CatalogRevision != current.CatalogRevision {
		return FarmQueuePlan{}, &SupervisorCommandError{Code: SupervisorReasonStateChanged}
	}
	if !strings.EqualFold(strings.TrimSpace(request.Character), strings.TrimSpace(current.Character)) ||
		!strings.EqualFold(strings.TrimSpace(request.Difficulty), strings.TrimSpace(current.Difficulty)) {
		return FarmQueuePlan{}, &QueueValidationError{Code: QueueReasonContextMismatch, EntryIndex: -1}
	}
	report, err := ResolveRunAvailabilities(cfg, RunAvailabilityContext{
		Character: current.Character, Difficulty: current.Difficulty, GameVersion: cfg.Memory.GameVersion,
	})
	if err != nil {
		return FarmQueuePlan{}, fmt.Errorf("resolve farm queue availability: %w", err)
	}
	available := make(map[string]tasks.RunAvailability, len(report.Runs))
	for _, entry := range report.Runs {
		available[string(entry.RunID)] = entry
	}
	for i, id := range queue {
		entry, ok := available[id]
		if !ok {
			return FarmQueuePlan{}, &QueueValidationError{Code: QueueReasonEntryUnavailable, EntryIndex: i, RunID: id, Reasons: []tasks.RunReason{tasks.RunReasonUnknown}}
		}
		if entry.Status == tasks.RunAvailabilityUnavailable {
			return FarmQueuePlan{}, &QueueValidationError{Code: QueueReasonEntryUnavailable, EntryIndex: i, RunID: id, Reasons: append([]tasks.RunReason(nil), entry.Reasons...)}
		}
	}
	budgets := FarmQueueBudgets{
		MaxRuns: cfg.Session.MaxRuns, MaxDuration: time.Duration(cfg.Session.MaxDurationMs) * time.Millisecond,
		MaxConsecutiveFailures: cfg.Session.MaxConsecutiveFailures, MaxTotalRestarts: cfg.Session.MaxTotalRestarts,
	}
	if err := validateFarmQueueBudgets(budgets); err != nil {
		return FarmQueuePlan{}, err
	}
	return FarmQueuePlan{
		RunIDs: queue, Character: current.Character, Difficulty: current.Difficulty,
		CatalogRevision: current.CatalogRevision, Budgets: budgets,
	}, nil
}

func validateUniqueQueueRunIDs(runIDs []string) ([]string, error) {
	queue := make([]string, len(runIDs))
	seen := make(map[string]int, len(runIDs))
	for i, rawID := range runIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return nil, &QueueValidationError{Code: QueueReasonEntryUnavailable, EntryIndex: i}
		}
		if first, duplicate := seen[id]; duplicate {
			return nil, &QueueValidationError{Code: QueueReasonDuplicateRun, EntryIndex: i, FirstIndex: first, RunID: id}
		}
		seen[id] = i
		queue[i] = id
	}
	return queue, nil
}

func validateFarmQueueBudgets(budgets FarmQueueBudgets) error {
	if budgets.MaxRuns <= 0 || budgets.MaxDuration <= 0 {
		return fmt.Errorf("farm queue run and duration budgets must be positive")
	}
	if budgets.MaxConsecutiveFailures < 0 || budgets.MaxTotalRestarts < 0 {
		return fmt.Errorf("farm queue failure and restart budgets must not be negative")
	}
	return nil
}
