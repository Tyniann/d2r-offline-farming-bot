package api

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

func TestHistoryAPIProjectsSameAnalysisAcrossEndpointsAndJSONExport(t *testing.T) {
	server, backend := startAPITestServer(t)
	backend.history = historyAPIFixture()
	query := "?difficulty=nightmare&timezone=Europe%2FVienna"

	var summary HistorySummaryResponse
	getHistoryJSON(t, server.URL()+"/api/v1/history/summary"+query, &summary)
	var comparisons HistoryComparisonsResponse
	getHistoryJSON(t, server.URL()+"/api/v1/history/comparisons"+query, &comparisons)
	var items HistoryItemsResponse
	getHistoryJSON(t, server.URL()+"/api/v1/history/items"+query, &items)
	var runs HistoryRunsResponse
	getHistoryJSON(t, server.URL()+"/api/v1/history/runs"+query, &runs)
	var report HistoryReportDTO
	response := getHistoryJSONResponse(t, server.URL()+"/api/v1/history/export"+query+"&format=json", &report)
	defer response.Body.Close()
	if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("JSON content type=%q", contentType)
	}

	if summary.Summary.TerminalRuns != report.Summary.TerminalRuns || len(summary.DailyBuckets) != len(report.DailyBuckets) || len(comparisons.Comparisons) != len(report.Comparisons) || len(items.Items) != len(report.Items) || len(runs.Runs) != len(report.Runs) {
		t.Fatalf("endpoint/export parity failed: summary=%+v comparisons=%d/%d items=%d/%d runs=%d/%d", summary.Summary, len(comparisons.Comparisons), len(report.Comparisons), len(items.Items), len(report.Items), len(runs.Runs), len(report.Runs))
	}
	if summary.Meta.IndexGeneration != 7 || summary.Meta.Timezone != "Europe/Vienna" || len(summary.Meta.Diagnostics) != 1 || len(backend.historyFilter.Difficulties) != 1 || backend.historyFilter.Difficulties[0] != "nightmare" || backend.historyFilter.Timezone != "Europe/Vienna" {
		t.Fatalf("meta/filter=%+v backend=%+v", summary.Meta, backend.historyFilter)
	}
	if runs.Runs[1].Reason != "boss_not_found" {
		t.Fatalf("reason=%q, want stable reason code", runs.Runs[1].Reason)
	}
	if disposition := response.Header.Get("Content-Disposition"); !strings.HasPrefix(disposition, `attachment; filename="d2r-history-`) || strings.Contains(disposition, "countess") {
		t.Fatalf("unsafe JSON disposition=%q", disposition)
	}
	csvResponse, err := http.Get(server.URL() + "/api/v1/history/export" + query + "&format=csv&dataset=runs")
	if err != nil {
		t.Fatal(err)
	}
	defer csvResponse.Body.Close()
	rows, err := csv.NewReader(csvResponse.Body).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows)-1 != len(report.Runs) {
		t.Fatalf("CSV run population=%d, JSON=%d", len(rows)-1, len(report.Runs))
	}
	var durationMs, bossKills, keepReturn, sold, pickupLost, postPickupLost int
	for _, row := range rows[1:] {
		values := []*int{&durationMs, &bossKills, &keepReturn, &sold, &pickupLost, &postPickupLost}
		for index, column := range []int{8, 9, 10, 11, 12, 13} {
			parsed, parseErr := strconv.Atoi(row[column])
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			*values[index] += parsed
		}
	}
	if int64(durationMs) != report.Summary.Durations.TotalMs || bossKills != report.Summary.BossKills || keepReturn != report.Summary.Funnel.KeepReturn || sold != report.Summary.Funnel.Sold || pickupLost != report.Summary.Funnel.PickupLost || postPickupLost != report.Summary.Funnel.PostPickupLost {
		t.Fatalf("CSV sums duration=%d boss=%d keep=%d sold=%d pickup_lost=%d post_pickup_lost=%d; JSON=%+v", durationMs, bossKills, keepReturn, sold, pickupLost, postPickupLost, report.Summary)
	}
}

func TestHistoryAPIPaginatesStablyAndRejectsStaleOrChangedCursor(t *testing.T) {
	server, backend := startAPITestServer(t)
	backend.history = historyAPIFixture()
	var first HistoryRunsResponse
	getHistoryJSON(t, server.URL()+"/api/v1/history/runs?limit=4", &first)
	if len(first.Runs) != 4 || first.NextCursor == "" {
		t.Fatalf("first page=%+v", first)
	}
	var second HistoryRunsResponse
	getHistoryJSON(t, server.URL()+"/api/v1/history/runs?limit=4&cursor="+url.QueryEscape(first.NextCursor), &second)
	if len(second.Runs) != 3 || second.Runs[0].RunID == first.Runs[0].RunID || second.NextCursor != "" {
		t.Fatalf("second page=%+v", second)
	}
	assertHistoryAPIError(t, server.URL()+"/api/v1/history/runs?limit=4&run=countess&cursor="+url.QueryEscape(first.NextCursor), http.StatusBadRequest, string(telemetry.HistoryReasonCursorInvalid))
	backend.history.snapshot.Generation++
	assertHistoryAPIError(t, server.URL()+"/api/v1/history/runs?limit=4&cursor="+url.QueryEscape(first.NextCursor), http.StatusBadRequest, string(telemetry.HistoryReasonCursorInvalid))
}

func TestHistoryAPIAcceptsSessionFilter(t *testing.T) {
	server, backend := startAPITestServer(t)
	backend.history = historyAPIFixture()
	var summary HistorySummaryResponse
	getHistoryJSON(t, server.URL()+"/api/v1/history/summary?session=session-keep", &summary)
	if len(backend.historyFilter.SessionIDs) != 1 || backend.historyFilter.SessionIDs[0] != "session-keep" || len(summary.Meta.Filter.SessionIDs) != 1 || summary.Meta.Filter.SessionIDs[0] != "session-keep" {
		t.Fatalf("session filter backend=%+v echo=%+v", backend.historyFilter, summary.Meta.Filter)
	}
}

func TestHistoryAPIForwardsSupportedComparisonSort(t *testing.T) {
	server, backend := startAPITestServer(t)
	backend.history = historyAPIFixture()
	var response HistoryComparisonsResponse
	getHistoryJSON(t, server.URL()+"/api/v1/history/comparisons?sort=average_duration", &response)
	if backend.historyFilter.Sort != telemetry.HistorySortAverageDuration || response.Meta.Filter.Sort != telemetry.HistorySortAverageDuration {
		t.Fatalf("sort backend=%q echo=%q", backend.historyFilter.Sort, response.Meta.Filter.Sort)
	}
}

func TestHistoryAPIDetailRawEventsAndNotFound(t *testing.T) {
	server, backend := startAPITestServer(t)
	backend.history = historyAPIFixture()
	var detail HistoryRunDetailResponse
	getHistoryJSON(t, server.URL()+"/api/v1/history/runs/countess-a?include_raw=true", &detail)
	if detail.Run.RunID != "countess-a" || detail.Run.RawContext["run_id"] != "countess-a" || len(detail.Run.RawEvents) != 1 || detail.Run.Items == nil {
		t.Fatalf("detail=%+v", detail)
	}
	if _, exists := detail.Run.RawContext["event"]; exists {
		t.Fatalf("raw context contains event payload: %+v", detail.Run.RawContext)
	}
	if detail.Run.RawEvents[0].RunID != "" || detail.Run.RawEvents[0].SchemaVersion != 0 {
		t.Fatalf("raw event repeats common context: %+v", detail.Run.RawEvents[0])
	}
	assertHistoryAPIError(t, server.URL()+"/api/v1/history/runs/missing", http.StatusNotFound, string(telemetry.HistoryReasonRunNotFound))
}

func TestHistoryCSVUsesStableColumnsAndNeutralizesFormulas(t *testing.T) {
	server, backend := startAPITestServer(t)
	backend.history = historyAPIFixture()
	response, err := http.Get(server.URL() + "/api/v1/history/export?format=csv&dataset=items")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	rows, err := csv.NewReader(response.Body).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/csv") || len(rows) != 2 || strings.Join(rows[0], ",") != strings.Join(telemetry.HistoryItemCSVColumns(), ",") || rows[1][1] != "'=HYPERLINK(\"bad\")" {
		t.Fatalf("response=%d headers=%v rows=%v", response.StatusCode, response.Header, rows)
	}
	if disposition := response.Header.Get("Content-Disposition"); !strings.HasPrefix(disposition, `attachment; filename="d2r-history-items-`) || strings.ContainsAny(disposition, "/\\") {
		t.Fatalf("unsafe CSV disposition=%q", disposition)
	}
}

func TestHistoryAPIRejectsInvalidFiltersLimitsSortsAndMethods(t *testing.T) {
	server, backend := startAPITestServer(t)
	backend.history = historyAPIFixture()
	for _, path := range []string{
		"/api/v1/history/summary?from=2026-07-20T10:00:00%2B02:00",
		"/api/v1/history/summary?unknown=true",
		"/api/v1/history/summary?session=session-keep&limit=200",
		"/api/v1/history/summary?timezone=Europe%2FDoes-Not-Exist",
		"/api/v1/history/runs?limit=201",
		"/api/v1/history/runs?sort=keep_per_hour",
		"/api/v1/history/export?format=csv&dataset=unknown",
	} {
		response, err := http.Get(server.URL() + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s status=%d", path, response.StatusCode)
		}
	}
	request, _ := http.NewRequest(http.MethodPost, server.URL()+"/api/v1/history/summary", strings.NewReader("{}"))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("history mutation status=%d", response.StatusCode)
	}
	request, _ = http.NewRequest(http.MethodGet, server.URL()+"/api/v1/history/summary", nil)
	request.Header.Set("Origin", "https://evil.example")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign-origin history status=%d", response.StatusCode)
	}
	request, _ = http.NewRequest(http.MethodGet, server.URL()+"/api/v1/history/summary", nil)
	request.Host = "evil.example"
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong-host history status=%d", response.StatusCode)
	}
}

func historyAPIFixture() historyData {
	started := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	countessRouteAKeepPerHour, countessRouteBKeepPerHour, mephistoKeepPerHour := 20.0, 40.0, 40.0
	runs := []telemetry.HistoryRunAnalysis{
		{RunID: "countess-a", StartedAt: started, ObservedAt: started.Add(time.Minute), Character: "MrBones", Difficulty: "nightmare", Run: "countess", DefinitionID: "countess", RouteID: "countess-route-a", Outcome: telemetry.HistoryOutcomeSuccess, DurationMs: 60_000, BossKills: 1, Funnel: telemetry.HistoryFunnel{Seen: 2, Matched: 2, PickedUp: 2, Stashed: 1, Sold: 1, KeepReturn: 1}},
		{RunID: "countess-failed", StartedAt: started.Add(time.Hour), ObservedAt: started.Add(time.Hour + 2*time.Minute), Character: "MrBones", Difficulty: "nightmare", Run: "countess", DefinitionID: "countess", RouteID: "countess-route-a", Outcome: telemetry.HistoryOutcomeFailed, Reason: "boss_not_found", LastStep: "acquire_boss", DurationMs: 120_000},
		{RunID: "countess-route-b", StartedAt: started.Add(2 * time.Hour), ObservedAt: started.Add(2*time.Hour + 90*time.Second), Character: "MrBones", Difficulty: "nightmare", Run: "countess", DefinitionID: "countess", RouteID: "countess-route-b", Outcome: telemetry.HistoryOutcomeSuccess, DurationMs: 90_000, BossKills: 1, Funnel: telemetry.HistoryFunnel{Seen: 1, Matched: 1, PickedUp: 1, Stashed: 1, KeepReturn: 1}},
		{RunID: "mephisto-failed", StartedAt: started.Add(3 * time.Hour), ObservedAt: started.Add(3*time.Hour + time.Minute), Character: "MrBones", Difficulty: "nightmare", Run: "mephisto", DefinitionID: "mephisto", RouteID: "mephisto-route-a", Outcome: telemetry.HistoryOutcomeFailed, Reason: "town_timeout", LastStep: "prepare_town_handoff", DurationMs: 60_000, BossKills: 1, Funnel: telemetry.HistoryFunnel{Seen: 2, Matched: 2, PickedUp: 1, PickupLost: 1, PostPickupLost: 1}},
		{RunID: "mephisto-success", StartedAt: started.Add(4 * time.Hour), ObservedAt: started.Add(4*time.Hour + 30*time.Second), Character: "MrBones", Difficulty: "nightmare", Run: "mephisto", DefinitionID: "mephisto", RouteID: "mephisto-route-a", Outcome: telemetry.HistoryOutcomeSuccess, DurationMs: 30_000, BossKills: 1, Funnel: telemetry.HistoryFunnel{Seen: 1, Matched: 1, PickedUp: 1, Stashed: 1, KeepReturn: 1}},
		{RunID: "active", StartedAt: started.Add(5 * time.Hour), ObservedAt: started.Add(5*time.Hour + time.Minute), Character: "MrBones", Difficulty: "nightmare", Run: "countess", DefinitionID: "countess", RouteID: "countess-route-a", Outcome: telemetry.HistoryOutcomeRunning},
		{RunID: "incomplete", StartedAt: started.Add(6 * time.Hour), ObservedAt: started.Add(6*time.Hour + time.Minute), Character: "MrBones", Difficulty: "nightmare", Run: "countess", DefinitionID: "countess", RouteID: "countess-route-a", Outcome: telemetry.HistoryOutcomeIncomplete},
	}
	return historyData{
		analysis: telemetry.HistoryAnalysis{
			Filter:       telemetry.HistoryFilter{Timezone: "Europe/Vienna", Difficulties: []string{"nightmare"}},
			Summary:      telemetry.HistorySummary{Runs: 7, TerminalRuns: 5, Successful: 3, Failed: 2, Running: 1, Incomplete: 1, SuccessRate: float64Pointer(0.6), BossKills: 4, Durations: telemetry.HistoryDurationStats{Count: 5, TotalMs: 360_000, AverageMs: 72_000, MedianMs: 60_000, MinimumMs: 30_000, MaximumMs: 120_000}, Funnel: telemetry.HistoryFunnel{Seen: 6, Matched: 6, PickedUp: 5, Stashed: 3, Sold: 1, KeepReturn: 3, PickupLost: 1, PostPickupLost: 1}, KeepPerRun: float64Pointer(0.6), KeepPerKill: float64Pointer(0.75), KeepPerHour: float64Pointer(30)},
			DailyBuckets: []telemetry.HistoryDailyBucket{{Date: "2026-07-20", StartUTC: started.Add(-8 * time.Hour), EndUTC: started.Add(16 * time.Hour), TerminalRuns: 5, Successful: 3, SuccessRate: float64Pointer(0.6), ActiveDurationMs: 360_000, ActiveHours: 0.1, KeepReturn: 3, KeepPerHour: float64Pointer(30)}},
			Comparisons: []telemetry.HistoryComparison{
				{ID: "countess-a", Character: "MrBones", Difficulty: "nightmare", DefinitionID: "countess", Run: "countess", RouteID: "countess-route-a", TerminalRuns: 2, Successful: 1, Failed: 1, SuccessRate: float64Pointer(0.5), BossKills: 1, LowSample: true, KeepPerHour: &countessRouteAKeepPerHour},
				{ID: "countess-b", Character: "MrBones", Difficulty: "nightmare", DefinitionID: "countess", Run: "countess", RouteID: "countess-route-b", TerminalRuns: 1, Successful: 1, SuccessRate: float64Pointer(1), BossKills: 1, LowSample: true, KeepPerHour: &countessRouteBKeepPerHour},
				{ID: "mephisto-a", Character: "MrBones", Difficulty: "nightmare", DefinitionID: "mephisto", Run: "mephisto", RouteID: "mephisto-route-a", TerminalRuns: 2, Successful: 1, Failed: 1, SuccessRate: float64Pointer(0.5), BossKills: 2, LowSample: true, KeepPerHour: &mephistoKeepPerHour},
			},
			Items: []telemetry.HistoryItemAggregate{{ItemKey: "base:r01:normal", ItemName: `=HYPERLINK("bad")`, Seen: 2, Matched: 2, PickedUp: 1, Stashed: 1, Sold: 1, PickupLost: 1, PostPickupLost: 1, YieldPerHour: &mephistoKeepPerHour}},
			Runs:  runs,
		},
		snapshot: telemetry.HistorySnapshot{Generation: 7, Diagnostics: []telemetry.HistoryFileDiagnostic{{File: "broken.jsonl", Code: telemetry.HistoryReasonFileInvalid}}, Runs: []telemetry.HistoryRun{{RunID: "countess-a", Events: []telemetry.Event{{SchemaVersion: telemetry.HistorySchemaVersion, Stream: telemetry.HistoryStreamRun, Event: telemetry.RunContext, RunID: "countess-a", Mode: telemetry.HistoryModeProductiveFarming, Run: "countess"}}}}},
	}
}

func float64Pointer(value float64) *float64 { return &value }

func getHistoryJSON(t *testing.T, endpoint string, target any) {
	t.Helper()
	response := getHistoryJSONResponse(t, endpoint, target)
	response.Body.Close()
}

func getHistoryJSONResponse(t *testing.T, endpoint string, target any) *http.Response {
	t.Helper()
	response, err := http.Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("GET %s status=%d body=%s", endpoint, response.StatusCode, body)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	return response
}

func assertHistoryAPIError(t *testing.T, endpoint string, status int, code string) {
	t.Helper()
	response, err := http.Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body ErrorDTO
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status || body.Code != code || body.Params != nil || body.RequestID == "" {
		t.Fatalf("endpoint=%s status=%d body=%+v", endpoint, response.StatusCode, body)
	}
}
