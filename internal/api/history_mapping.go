package api

import (
	"sort"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

type historyData struct {
	analysis telemetry.HistoryAnalysis
	snapshot telemetry.HistorySnapshot
}

type historyBackend interface {
	History(telemetry.HistoryFilter) (historyData, error)
}

func historyMeta(data historyData, generatedAt time.Time) HistoryMetaDTO {
	diagnostics := make([]HistoryDiagnosticDTO, 0, len(data.snapshot.Diagnostics)+len(data.analysis.Diagnostics))
	for _, diagnostic := range data.snapshot.Diagnostics {
		diagnostics = append(diagnostics, HistoryDiagnosticDTO{File: diagnostic.File, Code: string(diagnostic.Code), Message: diagnostic.Message})
	}
	for _, diagnostic := range data.analysis.Diagnostics {
		diagnostics = append(diagnostics, HistoryDiagnosticDTO{File: diagnostic.File, Code: string(diagnostic.Code), Message: diagnostic.Message})
	}
	return HistoryMetaDTO{
		SchemaVersion: schemaVersion, GeneratedAt: generatedAt.UTC(), Timezone: data.analysis.Filter.Timezone,
		IndexGeneration: data.snapshot.Generation, Filter: historyFilterDTO(data.analysis.Filter),
		Diagnostics: diagnostics, IgnoredFiles: data.snapshot.IgnoredFiles,
	}
}

func historyFilterDTO(filter telemetry.HistoryFilter) HistoryFilterDTO {
	return HistoryFilterDTO{
		FromUTC: filter.FromUTC, ToUTC: filter.ToUTC, Timezone: filter.Timezone,
		Runs: append([]string(nil), filter.Runs...), Characters: append([]string(nil), filter.Characters...),
		Difficulties: append([]string(nil), filter.Difficulties...), Outcomes: append([]telemetry.HistoryOutcome(nil), filter.Outcomes...),
		Reasons: append([]string(nil), filter.Reasons...), PickitProfiles: append([]string(nil), filter.PickitProfiles...),
		Sort: filter.Sort,
	}
}

func historyDailyBucketsDTO(values []telemetry.HistoryDailyBucket) []HistoryDailyBucketDTO {
	out := make([]HistoryDailyBucketDTO, len(values))
	for index, value := range values {
		out[index] = HistoryDailyBucketDTO{
			Date: value.Date, StartUTC: value.StartUTC, EndUTC: value.EndUTC,
			TerminalRuns: value.TerminalRuns, Successful: value.Successful,
			SuccessRate: cloneFloat(value.SuccessRate), ActiveDurationMs: value.ActiveDurationMs,
			ActiveHours: value.ActiveHours, KeepReturn: value.KeepReturn, KeepPerHour: cloneFloat(value.KeepPerHour),
		}
	}
	return out
}

func historySummaryDTO(value telemetry.HistorySummary) HistorySummaryDTO {
	return HistorySummaryDTO{
		Runs: value.Runs, TerminalRuns: value.TerminalRuns, Successful: value.Successful, Failed: value.Failed,
		Aborted: value.Aborted, Incomplete: value.Incomplete, Running: value.Running, SuccessRate: cloneFloat(value.SuccessRate),
		BossKills: value.BossKills, Durations: historyDurationDTO(value.Durations), Stages: historyStagesDTO(value.Stages),
		Funnel: historyFunnelDTO(value.Funnel), KeepPerRun: cloneFloat(value.KeepPerRun), KeepPerKill: cloneFloat(value.KeepPerKill),
		KeepPerHour: cloneFloat(value.KeepPerHour), TopFailure: historyFailureDTO(value.TopFailure),
	}
}

func historyDurationDTO(value telemetry.HistoryDurationStats) HistoryDurationDTO {
	return HistoryDurationDTO{Count: value.Count, TotalMs: value.TotalMs, AverageMs: value.AverageMs, MedianMs: value.MedianMs, MinimumMs: value.MinimumMs, MaximumMs: value.MaximumMs}
}

func historyStagesDTO(value telemetry.HistoryStageDurations) HistoryStagesDTO {
	return HistoryStagesDTO{TravelMs: value.TravelMs, CombatMs: value.CombatMs, LootMs: value.LootMs, ReturnTownMs: value.ReturnTownMs, OtherMs: value.OtherMs}
}

func historyFunnelDTO(value telemetry.HistoryFunnel) HistoryFunnelDTO {
	return HistoryFunnelDTO{Seen: value.Seen, Matched: value.Matched, PickedUp: value.PickedUp, Stashed: value.Stashed, Sold: value.Sold, KeepReturn: value.KeepReturn, PickupLost: value.PickupLost, PostPickupLost: value.PostPickupLost}
}

func historyFailureDTO(value *telemetry.HistoryFailure) *HistoryFailureDTO {
	if value == nil {
		return nil
	}
	return &HistoryFailureDTO{Step: value.Step, Reason: value.Reason, ReasonMessage: historyReasonText(value.Reason), Count: value.Count, LostDurationMs: value.LostDurationMs}
}

func historyComparisonsDTO(values []telemetry.HistoryComparison) []HistoryComparisonDTO {
	out := make([]HistoryComparisonDTO, len(values))
	for index, value := range values {
		out[index] = HistoryComparisonDTO{
			ID: value.ID, Character: value.Character, Difficulty: value.Difficulty, DefinitionID: value.DefinitionID,
			Run: value.Run, RouteID: value.RouteID, TerminalRuns: value.TerminalRuns, Successful: value.Successful,
			Failed: value.Failed, Aborted: value.Aborted, SuccessRate: cloneFloat(value.SuccessRate), BossKills: value.BossKills,
			LowSample: value.LowSample, Durations: historyDurationDTO(value.Durations), Stages: historyStagesDTO(value.Stages),
			Funnel: historyFunnelDTO(value.Funnel), KeepPerRun: cloneFloat(value.KeepPerRun), KeepPerKill: cloneFloat(value.KeepPerKill),
			KeepPerHour: cloneFloat(value.KeepPerHour), TopFailure: historyFailureDTO(value.TopFailure),
		}
	}
	return out
}

func historyItemsDTO(values []telemetry.HistoryItemAggregate) []HistoryItemDTO {
	out := make([]HistoryItemDTO, len(values))
	for index, value := range values {
		out[index] = HistoryItemDTO{
			ItemKey: value.ItemKey, ItemName: value.ItemName, BaseCode: value.BaseCode, Quality: value.Quality,
			Seen: value.Seen, Matched: value.Matched, PickedUp: value.PickedUp, Stashed: value.Stashed, Sold: value.Sold,
			PickupLost: value.PickupLost, PostPickupLost: value.PostPickupLost,
			YieldPerRun: cloneFloat(value.YieldPerRun), YieldPerKill: cloneFloat(value.YieldPerKill), YieldPerHour: cloneFloat(value.YieldPerHour),
		}
	}
	return out
}

func historyRunsDTO(values []telemetry.HistoryRunAnalysis) []HistoryRunDTO {
	out := make([]HistoryRunDTO, len(values))
	for index, value := range values {
		out[index] = historyRunDTO(value)
	}
	return out
}

func historyRunDTO(value telemetry.HistoryRunAnalysis) HistoryRunDTO {
	return HistoryRunDTO{
		RunID: value.RunID, StartedAt: value.StartedAt, ObservedAt: value.ObservedAt,
		Character: value.Character, Difficulty: value.Difficulty, Run: value.Run, DefinitionID: value.DefinitionID,
		RouteID: value.RouteID, Outcome: value.Outcome, Reason: value.Reason, ReasonMessage: historyReasonText(value.Reason),
		LastStep: value.LastStep, DurationMs: value.DurationMs, BossKills: value.BossKills, Funnel: historyFunnelDTO(value.Funnel),
	}
}

func historyRunDetailDTO(value telemetry.HistoryRunAnalysis, includeRaw bool, snapshot telemetry.HistorySnapshot) HistoryRunDetailDTO {
	detail := HistoryRunDetailDTO{
		HistoryRunDTO: historyRunDTO(value), EndedAt: value.EndedAt,
		RouteLayoutFingerprint: value.RouteLayoutFingerprint, Stages: historyStagesDTO(value.Stages),
		Items: make([]HistoryRunItemDTO, len(value.Items)),
	}
	for index, item := range value.Items {
		detail.Items[index] = HistoryRunItemDTO{
			UnitID: item.UnitID, ItemKey: item.ItemKey, ItemName: item.ItemName, BaseCode: item.BaseCode, Quality: item.Quality,
			IdentityKind: item.IdentityKind, IdentityKey: item.IdentityKey, PickitProfileID: item.PickitProfileID,
			PickitRuleID: item.PickitRuleID, PickitAction: item.PickitAction, PickitProfileRevision: item.PickitProfileRevision,
			PickitAssignmentRevision: item.PickitAssignmentRevision, Seen: item.Seen, Matched: item.Matched,
			PickedUp: item.PickedUp, Stashed: item.Stashed, Sold: item.Sold, PickupLost: item.PickupLost, PostPickupLost: item.PostPickupLost,
		}
	}
	if includeRaw {
		for _, run := range snapshot.Runs {
			if run.RunID == value.RunID {
				if len(run.Events) > 0 {
					detail.RawContext = historyRawContextDTO(run.Events[0])
				}
				detail.RawEvents = make([]telemetry.Event, len(run.Events))
				for index, event := range run.Events {
					detail.RawEvents[index] = telemetry.CompactRunEvent(event)
				}
				break
			}
		}
	}
	return detail
}

func historyRawContextDTO(event telemetry.Event) map[string]any {
	context := map[string]any{
		"schema_version": event.SchemaVersion,
		"stream":         event.Stream,
		"run_id":         event.RunID,
		"mode":           event.Mode,
		"run":            event.Run,
	}
	for key, value := range map[string]string{
		"session_id": event.SessionID, "game_id": event.GameID, "character": event.Character,
		"difficulty": event.Difficulty, "game_version": event.GameVersion, "definition_id": event.DefinitionID,
		"phase": event.Phase, "route_id": event.RouteID, "route_layout_fingerprint": event.RouteLayoutFingerprint,
		"setup_route_id": event.SetupRouteID, "setup_route_layout_fingerprint": event.SetupRouteLayoutFingerprint,
	} {
		if value != "" {
			context[key] = value
		}
	}
	if event.QueueIndex != nil {
		context["queue_index"] = *event.QueueIndex
	}
	if event.QueueCycle != nil {
		context["queue_cycle"] = *event.QueueCycle
	}
	if event.RunStartedAt != nil {
		context["run_started_at"] = event.RunStartedAt
	}
	if len(event.PickitProfiles) != 0 {
		context["pickit_profiles"] = event.PickitProfiles
	}
	if event.PickitAssignmentRevision != 0 {
		context["pickit_assignment_revision"] = event.PickitAssignmentRevision
	}
	return context
}

func historyReasonText(reason string) string {
	if reason == "" {
		return ""
	}
	if message, ok := telemetry.HistoryReasonMessage(telemetry.HistoryReasonCode(reason)); ok {
		return message
	}
	return "Der Run endete mit dem Reason-Code „" + reason + "“."
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func sortUniqueStrings(values []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}
