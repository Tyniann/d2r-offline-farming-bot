package api

import (
	"reflect"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

func TestStorageCompactionHistoryAndSessionContract(t *testing.T) {
	for _, mode := range []string{"pending", "resumed", "full"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
			session, err := telemetry.NewSessionRecorderWithContext(dir, telemetry.SessionRecorderContext{SessionID: "session-storage", Mode: telemetry.HistoryModeProductiveFarming, Character: "Fixture", Difficulty: "hell", GameVersion: "3.2.92777"})
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			run, err := telemetry.NewRunRecorder(dir, telemetry.RunRecorderContext{RunID: "storage-run", SessionID: session.SessionID(), GameID: "game-1", Mode: telemetry.HistoryModeProductiveFarming, Character: "Fixture", Difficulty: "hell", GameVersion: "3.2.92777", Run: "cows", DefinitionID: "cows", RouteID: "fixture", StartedAt: start})
			if err != nil {
				t.Fatal(err)
			}
			defer run.Close()
			zero := 0
			if emitErr := session.Emit(telemetry.Event{Timestamp: start, Event: telemetry.RunStarted, RunID: run.RunID(), GameID: "game-1", Run: "cows", QueueIndex: &zero, QueueCycle: &zero}); emitErr != nil {
				t.Fatal(emitErr)
			}
			emit := func(event telemetry.Event) {
				t.Helper()
				event.Timestamp = start.Add(time.Second)
				if emitErr := run.Emit(event); emitErr != nil {
					t.Fatal(emitErr)
				}
			}
			for _, name := range []telemetry.EventName{telemetry.DropSeen, telemetry.PickitMatch, telemetry.PickupSuccess} {
				emit(telemetry.Event{Event: name, Stage: telemetry.HistoryStageLoot, UnitID: 101, ItemKey: "base:glr:normal", BaseCode: "glr", Quality: "normal", PickitAction: "keep"})
			}
			emit(telemetry.Event{Event: telemetry.RunStepStarted, Step: "compact_storage", Stage: telemetry.HistoryStageReturnTown})
			emit(telemetry.Event{Event: telemetry.StorageCompactionStarted, UnitID: 101, Code: "glr", Stage: telemetry.HistoryStageReturnTown})
			params := map[string]string{"item_code": "glr", "material_code": "gpr"}
			reason := ""
			if mode != "pending" {
				name, terminal := telemetry.StorageCompactionCompleted, telemetry.RunCompleted
				stepEvent := telemetry.RunStepCompleted
				if mode == "full" {
					name, terminal, stepEvent = telemetry.StorageCompactionFailed, telemetry.RunFailed, telemetry.RunStepFailed
					reason = "storage_gem_full"
				} else {
					emit(telemetry.Event{Event: telemetry.StorageCompactionRecipe, UnitID: 101, Code: "glr", Compaction: &telemetry.CompactionProgress{SourceCode: "glr", OutputCode: "gpr", SourceBefore: 99, SourceAfter: 96, OutputBefore: 5, OutputAfter: 6}, Stage: telemetry.HistoryStageReturnTown})
				}
				emit(telemetry.Event{Event: name, UnitID: 101, Code: "glr", Reason: reason, ReasonParams: params, Stage: telemetry.HistoryStageReturnTown})
				emit(telemetry.Event{Event: stepEvent, Step: "compact_storage", Reason: reason, ReasonParams: params, Stage: telemetry.HistoryStageReturnTown})
				if mode == "resumed" {
					emit(telemetry.Event{Event: telemetry.StashSuccess, Stage: telemetry.HistoryStageReturnTown, UnitID: 101, ItemKey: "base:glr:normal", BaseCode: "glr", Quality: "normal", PickitAction: "keep"})
				}
				if emitErr := session.Emit(telemetry.Event{Timestamp: start.Add(2 * time.Second), Event: terminal, RunID: run.RunID(), GameID: "game-1", Run: "cows", Reason: reason, ReasonParams: params}); emitErr != nil {
					t.Fatal(emitErr)
				}
			}
			index, err := telemetry.NewHistoryIndex(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err = index.Refresh(); err != nil {
				t.Fatal(err)
			}
			snapshot := index.Snapshot("")
			if len(snapshot.Diagnostics) != 0 || len(snapshot.Runs) != 1 {
				t.Fatalf("snapshot=%+v", snapshot)
			}
			analysis, err := telemetry.AnalyzeHistory(snapshot, telemetry.HistoryFilter{})
			if err != nil {
				t.Fatal(err)
			}
			server, backend := startAPITestServer(t)
			backend.history = historyData{analysis: analysis, snapshot: snapshot}
			var response HistoryRunsResponse
			getHistoryJSON(t, server.URL()+"/api/v1/history/runs", &response)
			row := response.Runs[0]
			var summary HistorySummaryResponse
			getHistoryJSON(t, server.URL()+"/api/v1/history/summary?session=session-storage", &summary)
			wantCompacted := []telemetry.CompactedMaterial(nil)
			if mode == "resumed" {
				wantCompacted = []telemetry.CompactedMaterial{{Code: "gpr", Count: 1}}
			}
			if !reflect.DeepEqual(summary.Summary.Compacted, wantCompacted) {
				t.Fatalf("compacted=%+v", summary.Summary.Compacted)
			}
			if row.Funnel.Seen != 1 || row.Funnel.Matched != 1 || row.Funnel.PickedUp != 1 || row.Funnel.Stashed != map[string]int{"pending": 0, "resumed": 1, "full": 0}[mode] {
				t.Fatalf("funnel=%+v", row.Funnel)
			}
			if mode == "full" {
				if row.Reason != reason || !reflect.DeepEqual(row.ReasonParams, params) {
					t.Fatalf("row=%+v", row)
				}
				live := &LiveBackend{status: StatusDTO{State: "idle", LifecyclePhase: "idle"}}
				live.UpdateSupervisor(app.SupervisorSnapshot{State: app.SupervisorStateStoppedError, LastResult: app.SupervisorRunResult{Disposition: app.QueueRunStop, Reason: reason, ReasonParams: params}})
				status := live.Status()
				if status.LastError.Params["material_code"] != "gpr" || status.LastResult.ReasonParams["item_code"] != "glr" {
					t.Fatalf("status=%+v", status)
				}
				status.LastResult.ReasonParams["material_code"] = "changed"
				if live.Status().LastResult.ReasonParams["material_code"] != "gpr" {
					t.Fatal("status shares parameter map")
				}
			}
		})
	}
}
