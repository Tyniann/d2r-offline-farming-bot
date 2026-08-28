package telemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// HistoryFilter ist der kanonische Core-Filter für alle Historienprojektionen.
type HistoryFilter struct {
	FromUTC        *time.Time
	ToUTC          *time.Time
	Timezone       string
	Runs           []string
	Characters     []string
	Difficulties   []string
	Outcomes       []HistoryOutcome
	Reasons        []string
	PickitProfiles []string
	SessionIDs     []string
	Sort           HistorySort
}

// HistoryDailyBucket enthält ausschließlich Core-berechnete Werte eines lokalen Kalendertags.
type HistoryDailyBucket struct {
	Date             string
	StartUTC         time.Time
	EndUTC           time.Time
	TerminalRuns     int
	Successful       int
	SuccessRate      *float64
	ActiveDurationMs int64
	ActiveHours      float64
	KeepReturn       int
	KeepPerHour      *float64
}

// HistoryDurationStats fasst ausschließlich terminale aktive Run-Zeit zusammen.
type HistoryDurationStats struct {
	Count     int
	TotalMs   int64
	AverageMs float64
	MedianMs  float64
	MinimumMs int64
	MaximumMs int64
}

// HistoryStageDurations enthält disjunkte Core-Stages und unverteilte Restzeit.
type HistoryStageDurations struct {
	TravelMs     int64
	CombatMs     int64
	LootMs       int64
	ReturnTownMs int64
	OtherMs      int64
}

// HistoryFunnel enthält unterschiedliche Item-Units je semantischer Stufe.
type HistoryFunnel struct {
	Seen           int
	Matched        int
	PickedUp       int
	Stashed        int
	Sold           int
	KeepReturn     int
	PickupLost     int
	PostPickupLost int
}

// HistoryFailure fasst eine terminale Fehlerstelle deterministisch zusammen.
type HistoryFailure struct {
	Step           string
	Reason         string
	Count          int
	LostDurationMs int64
}

// HistoryRunAnalysis ist der nutzerorientierte Drill-down eines korrelierten Runs.
type HistoryRunAnalysis struct {
	RunID                  string
	StartedAt              time.Time
	ObservedAt             time.Time
	EndedAt                *time.Time
	Character              string
	Difficulty             string
	Run                    string
	DefinitionID           string
	RouteID                string
	RouteLayoutFingerprint string
	Outcome                HistoryOutcome
	Reason                 string
	LastStep               string
	DurationMs             int64
	BossKills              int
	Stages                 HistoryStageDurations
	Funnel                 HistoryFunnel
	Items                  []HistoryRunItem
}

// HistoryRunItem beschreibt eine Unit-Kette innerhalb genau eines Runs.
type HistoryRunItem struct {
	UnitID                   uint32
	ItemKey                  string
	ItemName                 string
	BaseCode                 string
	Quality                  string
	IdentityKind             string
	IdentityKey              string
	PickitProfileID          string
	PickitRuleID             string
	PickitAction             string
	PickitProfileRevision    uint64
	PickitAssignmentRevision uint64
	Seen                     bool
	Matched                  bool
	PickedUp                 bool
	Stashed                  bool
	Sold                     bool
	PickupLost               bool
	PostPickupLost           bool
}

// HistorySummary enthält exakt eine gefilterte Historienpopulation.
type HistorySummary struct {
	Runs         int
	TerminalRuns int
	Successful   int
	Failed       int
	Aborted      int
	Incomplete   int
	Running      int
	SuccessRate  *float64
	BossKills    int
	Durations    HistoryDurationStats
	Stages       HistoryStageDurations
	Funnel       HistoryFunnel
	KeepPerRun   *float64
	KeepPerKill  *float64
	KeepPerHour  *float64
	TopFailure   *HistoryFailure
}

// HistoryComparison trennt Ergebnisse nach Charakter, Difficulty, Definition und Route.
type HistoryComparison struct {
	ID           string
	Character    string
	Difficulty   string
	DefinitionID string
	Run          string
	RouteID      string
	TerminalRuns int
	Successful   int
	Failed       int
	Aborted      int
	SuccessRate  *float64
	BossKills    int
	LowSample    bool
	Durations    HistoryDurationStats
	Stages       HistoryStageDurations
	Funnel       HistoryFunnel
	KeepPerRun   *float64
	KeepPerKill  *float64
	KeepPerHour  *float64
	TopFailure   *HistoryFailure
}

// HistoryItemAggregate fasst eine stabile Itemidentität der gefilterten Population zusammen.
type HistoryItemAggregate struct {
	ItemKey        string
	ItemName       string
	BaseCode       string
	Quality        string
	Seen           int
	Matched        int
	PickedUp       int
	Stashed        int
	Sold           int
	PickupLost     int
	PostPickupLost int
	YieldPerRun    *float64
	YieldPerKill   *float64
	YieldPerHour   *float64
}

// HistoryAnalysis ist die einzige Core-Autorität für Summary, Vergleiche, Items und Runs.
type HistoryAnalysis struct {
	Filter       HistoryFilter
	Summary      HistorySummary
	DailyBuckets []HistoryDailyBucket
	Comparisons  []HistoryComparison
	Items        []HistoryItemAggregate
	Runs         []HistoryRunAnalysis
	// Diagnostics lists per-run analysis defects that were isolated instead of failing the whole snapshot.
	Diagnostics []HistoryFileDiagnostic
}

// AnalyzeHistory filtert immutable Runs und berechnet alle Phase-14-Aggregate rein im Core.
func AnalyzeHistory(snapshot HistorySnapshot, filter HistoryFilter) (HistoryAnalysis, error) {
	filter = cloneHistoryFilter(filter)
	timezone, timezoneErr := NormalizeHistoryTimezone(filter.Timezone)
	if timezoneErr != nil {
		return HistoryAnalysis{}, timezoneErr
	}
	filter.Timezone = timezone
	if err := validateHistoryFilter(filter); err != nil {
		return HistoryAnalysis{}, err
	}
	analysis := HistoryAnalysis{Filter: filter}
	for _, run := range snapshot.Runs {
		if run.Mode != HistoryModeProductiveFarming || !historyRunMatches(run, filter) {
			continue
		}
		row, err := analyzeHistoryRun(run)
		if err != nil {
			file := run.RunFile
			if file == "" {
				file = run.RunID + ".jsonl"
			}
			code := HistoryErrorCode(err)
			analysis.Diagnostics = append(analysis.Diagnostics, HistoryFileDiagnostic{File: file, Code: code})
			continue
		}
		analysis.Runs = append(analysis.Runs, row)
	}
	sort.Slice(analysis.Runs, func(a, b int) bool {
		if analysis.Runs[a].StartedAt.Equal(analysis.Runs[b].StartedAt) {
			return analysis.Runs[a].RunID < analysis.Runs[b].RunID
		}
		return analysis.Runs[a].StartedAt.After(analysis.Runs[b].StartedAt)
	})
	analysis.Summary = summarizeHistoryRuns(analysis.Runs)
	analysis.DailyBuckets, timezoneErr = historyDailyBuckets(analysis.Runs, filter)
	if timezoneErr != nil {
		return HistoryAnalysis{}, timezoneErr
	}
	analysis.Comparisons = compareHistoryRuns(analysis.Runs, filter.Sort)
	analysis.Items = aggregateHistoryItems(analysis.Runs, analysis.Summary)
	return analysis, nil
}

// NormalizeHistoryTimezone validiert eine IANA-Zeitzone und normalisiert einen leeren Wert auf `UTC`.
func NormalizeHistoryTimezone(value string) (string, error) {
	if value == "" {
		return "UTC", nil
	}
	if strings.TrimSpace(value) != value || value == "Local" {
		return "", historyReadError(HistoryReasonTimezoneInvalid, "invalid history timezone")
	}
	if _, err := time.LoadLocation(value); err != nil {
		return "", historyReadError(HistoryReasonTimezoneInvalid, "unknown history timezone %q", value)
	}
	return value, nil
}

func validateHistoryFilter(filter HistoryFilter) error {
	if filter.FromUTC != nil && filter.ToUTC != nil && !filter.FromUTC.Before(*filter.ToUTC) {
		return historyReadError(HistoryReasonFilterInvalid, "history interval must be half-open and increasing")
	}
	for _, bound := range []*time.Time{filter.FromUTC, filter.ToUTC} {
		if bound != nil {
			_, offset := bound.Zone()
			if offset != 0 {
				return historyReadError(HistoryReasonFilterInvalid, "history interval must use UTC")
			}
		}
	}
	for _, value := range append(append(append(append(append([]string(nil), filter.Runs...), filter.Characters...), filter.Difficulties...), filter.Reasons...), filter.SessionIDs...) {
		if strings.TrimSpace(value) == "" {
			return historyReadError(HistoryReasonFilterInvalid, "history filter contains an empty value")
		}
	}
	for _, difficulty := range filter.Difficulties {
		if difficulty != "normal" && difficulty != "nightmare" && difficulty != "hell" {
			return historyReadError(HistoryReasonFilterInvalid, "unknown difficulty %q", difficulty)
		}
	}
	for _, outcome := range filter.Outcomes {
		switch outcome {
		case HistoryOutcomeSuccess, HistoryOutcomeFailed, HistoryOutcomeAborted, HistoryOutcomeIncomplete, HistoryOutcomeRunning:
		default:
			return historyReadError(HistoryReasonFilterInvalid, "unknown outcome %q", outcome)
		}
	}
	if filter.Sort != "" && filter.Sort != HistorySortKeepPerHour && filter.Sort != HistorySortSuccessRate && filter.Sort != HistorySortAverageDuration {
		return historyReadError(HistoryReasonFilterInvalid, "unknown history sort %q", filter.Sort)
	}
	return nil
}

func historyRunMatches(run HistoryRun, filter HistoryFilter) bool {
	if filter.FromUTC != nil && run.StartedAt.Before(*filter.FromUTC) || filter.ToUTC != nil && !run.StartedAt.Before(*filter.ToUTC) {
		return false
	}
	if !matchesStringFilter(run.Run, filter.Runs) || !matchesStringFilter(run.Character, filter.Characters) || !matchesStringFilter(run.Difficulty, filter.Difficulties) || !matchesStringFilter(run.Reason, filter.Reasons) || !matchesStringFilter(run.SessionID, filter.SessionIDs) || !matchesOutcomeFilter(run.Outcome, filter.Outcomes) {
		return false
	}
	if len(filter.PickitProfiles) > 0 {
		for _, event := range run.Events {
			if matchesStringFilter(event.PickitProfileID, filter.PickitProfiles) {
				return true
			}
			for _, profile := range event.PickitProfiles {
				if matchesStringFilter(profile.ID, filter.PickitProfiles) {
					return true
				}
			}
		}
		return false
	}
	return true
}

func historyDailyBuckets(runs []HistoryRunAnalysis, filter HistoryFilter) ([]HistoryDailyBucket, error) {
	location, err := time.LoadLocation(filter.Timezone)
	if err != nil {
		return nil, historyReadError(HistoryReasonTimezoneInvalid, "unknown history timezone %q", filter.Timezone)
	}
	var first, last time.Time
	if filter.FromUTC != nil {
		first = localDay(*filter.FromUTC, location)
	}
	if filter.ToUTC != nil {
		last = localDay(filter.ToUTC.Add(-time.Nanosecond), location)
	}
	for _, run := range runs {
		if !terminalHistoryOutcome(run.Outcome) {
			continue
		}
		day := localDay(run.StartedAt, location)
		if first.IsZero() || day.Before(first) {
			first = day
		}
		if last.IsZero() || day.After(last) {
			last = day
		}
	}
	if first.IsZero() || last.IsZero() || first.After(last) {
		return []HistoryDailyBucket{}, nil
	}
	buckets := make([]HistoryDailyBucket, 0, int(last.Sub(first).Hours()/24)+2)
	byDate := make(map[string]int)
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		next := day.AddDate(0, 0, 1)
		buckets = append(buckets, HistoryDailyBucket{
			Date: day.Format("2006-01-02"), StartUTC: day.UTC(), EndUTC: next.UTC(),
		})
		byDate[day.Format("2006-01-02")] = len(buckets) - 1
	}
	for _, run := range runs {
		if !terminalHistoryOutcome(run.Outcome) {
			continue
		}
		index, ok := byDate[run.StartedAt.In(location).Format("2006-01-02")]
		if !ok {
			continue
		}
		bucket := &buckets[index]
		bucket.TerminalRuns++
		if run.Outcome == HistoryOutcomeSuccess {
			bucket.Successful++
		}
		bucket.ActiveDurationMs += run.DurationMs
		bucket.KeepReturn += run.Funnel.KeepReturn
	}
	for index := range buckets {
		bucket := &buckets[index]
		bucket.SuccessRate = ratio(bucket.Successful, bucket.TerminalRuns)
		bucket.ActiveHours = float64(bucket.ActiveDurationMs) / float64(time.Hour/time.Millisecond)
		bucket.KeepPerHour = perHour(bucket.KeepReturn, bucket.ActiveDurationMs)
	}
	return buckets, nil
}

func localDay(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func matchesStringFilter(value string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func matchesOutcomeFilter(value HistoryOutcome, allowed []HistoryOutcome) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

type historyItemState struct {
	HistoryRunItem
}

func analyzeHistoryRun(run HistoryRun) (HistoryRunAnalysis, error) {
	row := HistoryRunAnalysis{
		RunID: run.RunID, StartedAt: run.StartedAt, ObservedAt: run.ObservedAt, Character: run.Character,
		Difficulty: run.Difficulty, Run: run.Run, DefinitionID: run.DefinitionID, RouteID: run.RouteID,
		RouteLayoutFingerprint: run.RouteLayoutFingerprint, Outcome: run.Outcome, Reason: run.Reason,
	}
	if run.EndedAt != nil {
		ended := *run.EndedAt
		row.EndedAt = &ended
		row.DurationMs = ended.Sub(run.StartedAt).Milliseconds()
	} else {
		row.DurationMs = run.ObservedAt.Sub(run.StartedAt).Milliseconds()
	}
	if row.DurationMs < 0 {
		return HistoryRunAnalysis{}, historyReadError(HistoryReasonTimeInvalid, "negative run duration")
	}
	items := make(map[uint32]*historyItemState)
	var activeStep *Event
	bossSeen := false
	for _, event := range run.Events {
		if event.Timestamp.Before(run.StartedAt) || run.EndedAt != nil && event.Timestamp.After(*run.EndedAt) {
			return HistoryRunAnalysis{}, historyReadError(HistoryReasonTimeInvalid, "event lies outside run lifetime")
		}
		switch event.Event {
		case BossKillConfirmed:
			if bossSeen {
				return HistoryRunAnalysis{}, historyReadError(HistoryReasonBossDuplicate, "duplicate boss kill")
			}
			bossSeen = true
			row.BossKills++
		case RunStepStarted:
			if activeStep != nil {
				return HistoryRunAnalysis{}, historyReadError(HistoryReasonStageInvalid, "run steps overlap")
			}
			row.LastStep = event.Step
			started := cloneHistoryEvent(event)
			activeStep = &started
		case RunStepCompleted, RunStepFailed:
			row.LastStep = event.Step
			if activeStep == nil || activeStep.Step != event.Step || activeStep.Stage != event.Stage || event.Timestamp.Before(activeStep.Timestamp) {
				return HistoryRunAnalysis{}, historyReadError(HistoryReasonStageInvalid, "invalid step interval")
			}
			addHistoryStageDuration(&row.Stages, event.Stage, event.Timestamp.Sub(activeStep.Timestamp).Milliseconds())
			if event.Event == RunStepFailed {
				row.LastStep = event.Step
			}
			activeStep = nil
		}
		if isHistoryItemEvent(event.Event) {
			state := items[event.UnitID]
			if state == nil {
				state = &historyItemState{HistoryRunItem: HistoryRunItem{UnitID: event.UnitID}}
				items[event.UnitID] = state
			}
			mergeHistoryRunItem(&state.HistoryRunItem, event)
		}
	}
	if run.EndedAt != nil && activeStep != nil {
		return HistoryRunAnalysis{}, historyReadError(HistoryReasonStageInvalid, "terminal run has an open step")
	}
	stageTotal := row.Stages.TravelMs + row.Stages.CombatMs + row.Stages.LootMs + row.Stages.ReturnTownMs
	if run.EndedAt != nil {
		if stageTotal > row.DurationMs {
			return HistoryRunAnalysis{}, historyReadError(HistoryReasonStageInvalid, "stage duration exceeds active run time")
		}
		row.Stages.OtherMs = row.DurationMs - stageTotal
	}
	unitIDs := make([]uint32, 0, len(items))
	for unitID := range items {
		unitIDs = append(unitIDs, unitID)
	}
	sort.Slice(unitIDs, func(a, b int) bool { return unitIDs[a] < unitIDs[b] })
	for _, unitID := range unitIDs {
		item := items[unitID].HistoryRunItem
		item.PickupLost = item.Matched && !item.PickedUp
		item.PostPickupLost = item.PickitAction == "keep" && item.PickedUp && !item.Stashed
		row.Items = append(row.Items, item)
		addItemToFunnel(&row.Funnel, item)
	}
	return row, nil
}

func mergeHistoryRunItem(item *HistoryRunItem, event Event) {
	if item.ItemKey == "" {
		item.ItemKey = event.ItemKey
	}
	if item.ItemName == "" {
		item.ItemName = event.ItemName
	}
	if item.BaseCode == "" {
		item.BaseCode = event.BaseCode
	}
	if item.Quality == "" {
		item.Quality = event.Quality
	}
	if item.IdentityKind == "" {
		item.IdentityKind = event.ItemIdentityKind
	}
	if item.IdentityKey == "" {
		item.IdentityKey = event.ItemIdentityKey
	}
	if item.PickitProfileID == "" {
		item.PickitProfileID = event.PickitProfileID
	}
	if item.PickitRuleID == "" {
		item.PickitRuleID = event.PickitRuleID
	}
	if item.PickitAction == "" {
		item.PickitAction = event.PickitAction
	}
	if item.PickitProfileRevision == 0 {
		item.PickitProfileRevision = event.PickitProfileRevision
	}
	if item.PickitAssignmentRevision == 0 {
		item.PickitAssignmentRevision = event.PickitAssignmentRevision
	}
	item.Seen = item.Seen || event.Event == DropSeen
	item.Matched = item.Matched || event.Event == PickitMatch
	item.PickedUp = item.PickedUp || event.Event == PickupSuccess
	item.Stashed = item.Stashed || event.Event == StashSuccess
	item.Sold = item.Sold || event.Event == SellSuccess
}

func addItemToFunnel(funnel *HistoryFunnel, item HistoryRunItem) {
	if item.Seen {
		funnel.Seen++
	}
	if item.Matched {
		funnel.Matched++
	}
	if item.PickedUp {
		funnel.PickedUp++
	}
	if item.Stashed {
		funnel.Stashed++
	}
	if item.PickitAction == "sell" && item.Matched && item.PickedUp && item.Sold {
		funnel.Sold++
	}
	if item.PickitAction == "keep" && item.Matched && item.PickedUp && item.Stashed {
		funnel.KeepReturn++
	}
	if item.PickupLost {
		funnel.PickupLost++
	}
	if item.PostPickupLost {
		funnel.PostPickupLost++
	}
}

func addHistoryStageDuration(stages *HistoryStageDurations, stage HistoryStage, duration int64) {
	switch stage {
	case HistoryStageTravel:
		stages.TravelMs += duration
	case HistoryStageCombat:
		stages.CombatMs += duration
	case HistoryStageLoot:
		stages.LootMs += duration
	case HistoryStageReturnTown:
		stages.ReturnTownMs += duration
	}
}

func summarizeHistoryRuns(runs []HistoryRunAnalysis) HistorySummary {
	summary := HistorySummary{Runs: len(runs)}
	durations := make([]int64, 0, len(runs))
	failures := make(map[string]*HistoryFailure)
	for _, run := range runs {
		switch run.Outcome {
		case HistoryOutcomeRunning:
			summary.Running++
			continue
		case HistoryOutcomeIncomplete:
			summary.Incomplete++
			continue
		case HistoryOutcomeSuccess:
			summary.Successful++
		case HistoryOutcomeFailed:
			summary.Failed++
		case HistoryOutcomeAborted:
			summary.Aborted++
		}
		summary.TerminalRuns++
		summary.BossKills += run.BossKills
		durations = append(durations, run.DurationMs)
		addHistoryStages(&summary.Stages, run.Stages)
		addHistoryFunnel(&summary.Funnel, run.Funnel)
		if run.Outcome == HistoryOutcomeFailed || run.Outcome == HistoryOutcomeAborted {
			addHistoryFailure(failures, run.LastStep, run.Reason, run.DurationMs)
		}
	}
	summary.Durations = calculateDurationStats(durations)
	summary.SuccessRate = ratio(summary.Successful, summary.TerminalRuns)
	summary.KeepPerRun = ratio(summary.Funnel.KeepReturn, summary.TerminalRuns)
	summary.KeepPerKill = ratio(summary.Funnel.KeepReturn, summary.BossKills)
	summary.KeepPerHour = perHour(summary.Funnel.KeepReturn, summary.Durations.TotalMs)
	summary.TopFailure = topHistoryFailure(failures)
	return summary
}

func compareHistoryRuns(runs []HistoryRunAnalysis, sortBy HistorySort) []HistoryComparison {
	groups := make(map[string][]HistoryRunAnalysis)
	for _, run := range runs {
		if !terminalHistoryOutcome(run.Outcome) {
			continue
		}
		key := strings.Join([]string{run.Character, run.Difficulty, run.DefinitionID, run.RouteID}, "\x1f")
		groups[key] = append(groups[key], run)
	}
	comparisons := make([]HistoryComparison, 0, len(groups))
	for key, group := range groups {
		summary := summarizeHistoryRuns(group)
		first := group[0]
		comparison := HistoryComparison{
			ID: stableHistoryComparisonID(key), Character: first.Character, Difficulty: first.Difficulty,
			DefinitionID: first.DefinitionID, Run: first.Run, RouteID: first.RouteID,
			TerminalRuns: summary.TerminalRuns, Successful: summary.Successful, Failed: summary.Failed, Aborted: summary.Aborted,
			SuccessRate: summary.SuccessRate, BossKills: summary.BossKills, LowSample: summary.BossKills < HistoryLowSampleBossKills,
			Durations: summary.Durations, Stages: summary.Stages, Funnel: summary.Funnel, TopFailure: summary.TopFailure,
		}
		comparison.KeepPerRun = ratio(summary.Funnel.KeepReturn, summary.TerminalRuns)
		comparison.KeepPerKill = ratio(summary.Funnel.KeepReturn, summary.BossKills)
		comparison.KeepPerHour = perHour(summary.Funnel.KeepReturn, summary.Durations.TotalMs)
		comparisons = append(comparisons, comparison)
	}
	sort.Slice(comparisons, func(a, b int) bool {
		if sortBy == HistorySortSuccessRate {
			return compareOptionalHistoryMetric(comparisons[a].SuccessRate, comparisons[b].SuccessRate, comparisons[a].ID, comparisons[b].ID, true)
		}
		if sortBy == HistorySortAverageDuration {
			if comparisons[a].Durations.AverageMs == comparisons[b].Durations.AverageMs {
				return comparisons[a].ID < comparisons[b].ID
			}
			return comparisons[a].Durations.AverageMs < comparisons[b].Durations.AverageMs
		}
		left, right := comparisons[a].KeepPerHour, comparisons[b].KeepPerHour
		return compareOptionalHistoryMetric(left, right, comparisons[a].ID, comparisons[b].ID, true)
	})
	return comparisons
}

func compareOptionalHistoryMetric(left, right *float64, leftID, rightID string, descending bool) bool {
	if left == nil || right == nil {
		if left == nil && right == nil {
			return leftID < rightID
		}
		return left != nil
	}
	if *left == *right {
		return leftID < rightID
	}
	if descending {
		return *left > *right
	}
	return *left < *right
}

func aggregateHistoryItems(runs []HistoryRunAnalysis, summary HistorySummary) []HistoryItemAggregate {
	items := make(map[string]*HistoryItemAggregate)
	yields := make(map[string]int)
	for _, run := range runs {
		if !terminalHistoryOutcome(run.Outcome) {
			continue
		}
		for _, runItem := range run.Items {
			if runItem.ItemKey == "" {
				continue
			}
			item := items[runItem.ItemKey]
			if item == nil {
				item = &HistoryItemAggregate{ItemKey: runItem.ItemKey, ItemName: runItem.ItemName, BaseCode: runItem.BaseCode, Quality: runItem.Quality}
				items[runItem.ItemKey] = item
			}
			if item.ItemName == "" {
				item.ItemName = runItem.ItemName
			}
			if runItem.Seen {
				item.Seen++
			}
			if runItem.Matched {
				item.Matched++
			}
			if runItem.PickedUp {
				item.PickedUp++
			}
			if runItem.Stashed {
				item.Stashed++
			}
			if runItem.PickitAction == "sell" && runItem.Matched && runItem.PickedUp && runItem.Sold {
				item.Sold++
			}
			if runItem.PickupLost {
				item.PickupLost++
			}
			if runItem.PostPickupLost {
				item.PostPickupLost++
			}
			if runItem.PickitAction == "keep" && runItem.Matched && runItem.PickedUp && runItem.Stashed {
				yields[runItem.ItemKey]++
			}
		}
	}
	out := make([]HistoryItemAggregate, 0, len(items))
	for _, item := range items {
		yield := yields[item.ItemKey]
		item.YieldPerRun = ratio(yield, summary.TerminalRuns)
		item.YieldPerKill = ratio(yield, summary.BossKills)
		item.YieldPerHour = perHour(yield, summary.Durations.TotalMs)
		out = append(out, *item)
	}
	sort.Slice(out, func(a, b int) bool {
		left, right := strings.ToLower(out[a].ItemName), strings.ToLower(out[b].ItemName)
		if left == right {
			return out[a].ItemKey < out[b].ItemKey
		}
		return left < right
	})
	return out
}

func calculateDurationStats(values []int64) HistoryDurationStats {
	stats := HistoryDurationStats{Count: len(values)}
	if len(values) == 0 {
		return stats
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a] < sorted[b] })
	for _, value := range sorted {
		stats.TotalMs += value
	}
	stats.AverageMs = float64(stats.TotalMs) / float64(len(sorted))
	middle := len(sorted) / 2
	if len(sorted)%2 == 0 {
		stats.MedianMs = float64(sorted[middle-1]+sorted[middle]) / 2
	} else {
		stats.MedianMs = float64(sorted[middle])
	}
	stats.MinimumMs, stats.MaximumMs = sorted[0], sorted[len(sorted)-1]
	return stats
}

func addHistoryStages(target *HistoryStageDurations, value HistoryStageDurations) {
	target.TravelMs += value.TravelMs
	target.CombatMs += value.CombatMs
	target.LootMs += value.LootMs
	target.ReturnTownMs += value.ReturnTownMs
	target.OtherMs += value.OtherMs
}

func addHistoryFunnel(target *HistoryFunnel, value HistoryFunnel) {
	target.Seen += value.Seen
	target.Matched += value.Matched
	target.PickedUp += value.PickedUp
	target.Stashed += value.Stashed
	target.Sold += value.Sold
	target.KeepReturn += value.KeepReturn
	target.PickupLost += value.PickupLost
	target.PostPickupLost += value.PostPickupLost
}

func addHistoryFailure(failures map[string]*HistoryFailure, step, reason string, duration int64) {
	key := step + "\x1f" + reason
	failure := failures[key]
	if failure == nil {
		failure = &HistoryFailure{Step: step, Reason: reason}
		failures[key] = failure
	}
	failure.Count++
	failure.LostDurationMs += duration
}

func topHistoryFailure(failures map[string]*HistoryFailure) *HistoryFailure {
	values := make([]HistoryFailure, 0, len(failures))
	for _, failure := range failures {
		values = append(values, *failure)
	}
	sort.Slice(values, func(a, b int) bool {
		if values[a].Count != values[b].Count {
			return values[a].Count > values[b].Count
		}
		if values[a].Step != values[b].Step {
			return values[a].Step < values[b].Step
		}
		return values[a].Reason < values[b].Reason
	})
	if len(values) == 0 {
		return nil
	}
	return &values[0]
}

func terminalHistoryOutcome(outcome HistoryOutcome) bool {
	return outcome == HistoryOutcomeSuccess || outcome == HistoryOutcomeFailed || outcome == HistoryOutcomeAborted
}

func ratio(numerator, denominator int) *float64 {
	if denominator == 0 {
		return nil
	}
	value := float64(numerator) / float64(denominator)
	return &value
}

func perHour(count int, durationMs int64) *float64 {
	if durationMs <= 0 {
		return nil
	}
	value := float64(count) / (float64(durationMs) / float64(time.Hour/time.Millisecond))
	return &value
}

func stableHistoryComparisonID(key string) string {
	hash := sha256.Sum256([]byte(key))
	return "comparison-" + hex.EncodeToString(hash[:8])
}

func cloneHistoryFilter(filter HistoryFilter) HistoryFilter {
	filter.Runs = append([]string(nil), filter.Runs...)
	filter.Characters = append([]string(nil), filter.Characters...)
	filter.Difficulties = append([]string(nil), filter.Difficulties...)
	filter.Outcomes = append([]HistoryOutcome(nil), filter.Outcomes...)
	filter.Reasons = append([]string(nil), filter.Reasons...)
	filter.PickitProfiles = append([]string(nil), filter.PickitProfiles...)
	filter.SessionIDs = append([]string(nil), filter.SessionIDs...)
	if filter.FromUTC != nil {
		value := *filter.FromUTC
		filter.FromUTC = &value
	}
	if filter.ToUTC != nil {
		value := *filter.ToUTC
		filter.ToUTC = &value
	}
	return filter
}
