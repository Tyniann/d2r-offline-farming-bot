package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

type historyCursor struct {
	Generation uint64 `json:"generation"`
	Dataset    string `json:"dataset"`
	QueryHash  string `json:"query_hash"`
	Offset     int    `json:"offset"`
}

func (s *Server) historyBackend(w http.ResponseWriter, r *http.Request) (historyBackend, bool) {
	backend, ok := s.backend.(historyBackend)
	if !ok {
		s.writeError(w, http.StatusNotImplemented, "history_unavailable", requestIDFrom(r), nil)
	}
	return backend, ok
}

func (s *Server) handleHistorySummary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	data, generatedAt, ok := s.historyData(w, r, historyQueryOptions{})
	if !ok {
		return
	}
	s.writeJSON(w, http.StatusOK, HistorySummaryResponse{
		Meta: historyMeta(data, generatedAt), Summary: historySummaryDTO(data.analysis.Summary),
		DailyBuckets: historyDailyBucketsDTO(data.analysis.DailyBuckets),
	})
}

func (s *Server) handleHistoryComparisons(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	data, generatedAt, ok := s.historyData(w, r, historyQueryOptions{sort: string(telemetry.HistorySortKeepPerHour), comparisonSort: true})
	if !ok {
		return
	}
	s.writeJSON(w, http.StatusOK, HistoryComparisonsResponse{Meta: historyMeta(data, generatedAt), Comparisons: historyComparisonsDTO(data.analysis.Comparisons)})
}

func (s *Server) handleHistoryItems(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	options := historyQueryOptions{sort: string(telemetry.HistorySortItemName), paginate: true, dataset: "items"}
	data, generatedAt, ok := s.historyData(w, r, options)
	if !ok {
		return
	}
	limit, cursor, ok := s.historyPage(w, r, data, options, len(data.analysis.Items))
	if !ok {
		return
	}
	end := min(cursor.Offset+limit, len(data.analysis.Items))
	response := HistoryItemsResponse{Meta: historyMeta(data, generatedAt), Items: historyItemsDTO(data.analysis.Items[cursor.Offset:end])}
	if end < len(data.analysis.Items) {
		response.NextCursor = encodeHistoryCursor(historyCursor{Generation: data.snapshot.Generation, Dataset: options.dataset, QueryHash: historyQueryHash(r.URL.Query(), options), Offset: end})
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleHistoryRuns(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	options := historyQueryOptions{sort: string(telemetry.HistorySortStartedAt), paginate: true, dataset: "runs"}
	data, generatedAt, ok := s.historyData(w, r, options)
	if !ok {
		return
	}
	limit, cursor, ok := s.historyPage(w, r, data, options, len(data.analysis.Runs))
	if !ok {
		return
	}
	end := min(cursor.Offset+limit, len(data.analysis.Runs))
	response := HistoryRunsResponse{Meta: historyMeta(data, generatedAt), Runs: historyRunsDTO(data.analysis.Runs[cursor.Offset:end])}
	if end < len(data.analysis.Runs) {
		response.NextCursor = encodeHistoryCursor(historyCursor{Generation: data.snapshot.Generation, Dataset: options.dataset, QueryHash: historyQueryHash(r.URL.Query(), options), Offset: end})
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleHistoryRunDetail(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	if unknownHistoryQuery(r.URL.Query(), map[string]bool{"include_raw": true}) != "" {
		s.writeHistoryError(w, r, telemetry.HistoryReasonFilterInvalid)
		return
	}
	includeRaw := false
	if value := r.URL.Query().Get("include_raw"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			s.writeHistoryError(w, r, telemetry.HistoryReasonFilterInvalid)
			return
		}
		includeRaw = parsed
	}
	backend, ok := s.historyBackend(w, r)
	if !ok {
		return
	}
	data, err := backend.History(telemetry.HistoryFilter{})
	if err != nil {
		s.writeHistoryBackendError(w, r, err)
		return
	}
	runID := r.PathValue("runID")
	for _, run := range data.analysis.Runs {
		if run.RunID == runID {
			generatedAt := time.Now().UTC()
			s.writeJSON(w, http.StatusOK, HistoryRunDetailResponse{Meta: historyMeta(data, generatedAt), Run: historyRunDetailDTO(run, includeRaw, data.snapshot)})
			return
		}
	}
	s.writeHistoryError(w, r, telemetry.HistoryReasonRunNotFound)
}

func (s *Server) handleHistoryExport(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	format := r.URL.Query().Get("format")
	if format != "json" && format != "csv" {
		s.writeHistoryError(w, r, telemetry.HistoryReasonExportInvalid)
		return
	}
	dataset := r.URL.Query().Get("dataset")
	if (format == "csv" && dataset != "runs" && dataset != "items") || (format == "json" && dataset != "") {
		s.writeHistoryError(w, r, telemetry.HistoryReasonExportInvalid)
		return
	}
	data, generatedAt, ok := s.historyData(w, r, historyQueryOptions{export: true})
	if !ok {
		return
	}
	stamp := generatedAt.Format("20060102T150405Z")
	if format == "json" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="d2r-history-%s.json"`, stamp))
		s.writeJSON(w, http.StatusOK, HistoryReportDTO{
			Meta: historyMeta(data, generatedAt), Summary: historySummaryDTO(data.analysis.Summary),
			DailyBuckets: historyDailyBucketsDTO(data.analysis.DailyBuckets), Comparisons: historyComparisonsDTO(data.analysis.Comparisons),
			Items: historyItemsDTO(data.analysis.Items), Runs: historyRunsDTO(data.analysis.Runs),
		})
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="d2r-history-%s-%s.csv"`, dataset, stamp))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	writer := csv.NewWriter(w)
	if dataset == "runs" {
		writeHistoryRunCSV(writer, data.analysis.Runs)
	} else {
		writeHistoryItemCSV(writer, data.analysis.Items)
	}
	writer.Flush()
}

type historyQueryOptions struct {
	sort           string
	comparisonSort bool
	paginate       bool
	dataset        string
	export         bool
}

func (s *Server) historyData(w http.ResponseWriter, r *http.Request, options historyQueryOptions) (historyData, time.Time, bool) {
	filter, err := parseHistoryFilter(r.URL.Query(), options)
	if err != nil {
		code := telemetry.HistoryErrorCode(err)
		if code != telemetry.HistoryReasonTimezoneInvalid {
			code = telemetry.HistoryReasonFilterInvalid
		}
		s.writeHistoryError(w, r, code)
		return historyData{}, time.Time{}, false
	}
	backend, ok := s.historyBackend(w, r)
	if !ok {
		return historyData{}, time.Time{}, false
	}
	data, err := backend.History(filter)
	if err != nil {
		s.writeHistoryBackendError(w, r, err)
		return historyData{}, time.Time{}, false
	}
	return data, time.Now().UTC(), true
}

func parseHistoryFilter(query url.Values, options historyQueryOptions) (telemetry.HistoryFilter, error) {
	allowed := map[string]bool{"from": true, "to": true, "timezone": true, "run": true, "character": true, "difficulty": true, "outcome": true, "reason": true, "pickit_profile": true}
	if options.sort != "" {
		allowed["sort"] = true
	}
	if options.paginate {
		allowed["limit"], allowed["cursor"] = true, true
	}
	if options.export {
		allowed["format"], allowed["dataset"] = true, true
	}
	if unknown := unknownHistoryQuery(query, allowed); unknown != "" {
		return telemetry.HistoryFilter{}, fmt.Errorf("%s: unknown query %q", telemetry.HistoryReasonFilterInvalid, unknown)
	}
	for _, key := range []string{"from", "to", "timezone", "sort", "limit", "cursor", "format", "dataset"} {
		if len(query[key]) > 1 {
			return telemetry.HistoryFilter{}, fmt.Errorf("%s: repeated query %q", telemetry.HistoryReasonFilterInvalid, key)
		}
	}
	requestedSort := query.Get("sort")
	if options.sort != "" && requestedSort != "" && requestedSort != options.sort {
		if !options.comparisonSort || (requestedSort != string(telemetry.HistorySortSuccessRate) && requestedSort != string(telemetry.HistorySortAverageDuration)) {
			return telemetry.HistoryFilter{}, fmt.Errorf("%s: unsupported sort", telemetry.HistoryReasonFilterInvalid)
		}
	}
	filter := telemetry.HistoryFilter{
		Timezone: query.Get("timezone"),
		Runs:     queryList(query, "run"), Characters: queryList(query, "character"), Difficulties: queryList(query, "difficulty"),
		Reasons: queryList(query, "reason"), PickitProfiles: queryList(query, "pickit_profile"),
	}
	timezone, timezoneErr := telemetry.NormalizeHistoryTimezone(filter.Timezone)
	if timezoneErr != nil {
		return telemetry.HistoryFilter{}, timezoneErr
	}
	filter.Timezone = timezone
	if options.comparisonSort {
		filter.Sort = telemetry.HistorySort(requestedSort)
		if filter.Sort == "" {
			filter.Sort = telemetry.HistorySortKeepPerHour
		}
	}
	for _, value := range queryList(query, "outcome") {
		filter.Outcomes = append(filter.Outcomes, telemetry.HistoryOutcome(value))
	}
	for key, target := range map[string]**time.Time{"from": &filter.FromUTC, "to": &filter.ToUTC} {
		if value := query.Get(key); value != "" {
			parsed, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return telemetry.HistoryFilter{}, fmt.Errorf("%s: invalid %s", telemetry.HistoryReasonFilterInvalid, key)
			}
			_, offset := parsed.Zone()
			if offset != 0 {
				return telemetry.HistoryFilter{}, fmt.Errorf("%s: %s must be UTC", telemetry.HistoryReasonFilterInvalid, key)
			}
			utc := parsed.UTC()
			*target = &utc
		}
	}
	return filter, nil
}

func unknownHistoryQuery(query url.Values, allowed map[string]bool) string {
	for key := range query {
		if !allowed[key] {
			return key
		}
	}
	return ""
}

func queryList(query url.Values, key string) []string {
	var values []string
	for _, raw := range query[key] {
		for _, value := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				values = append(values, trimmed)
			}
		}
	}
	return sortUniqueStrings(values)
}

func (s *Server) historyPage(w http.ResponseWriter, r *http.Request, data historyData, options historyQueryOptions, length int) (int, historyCursor, bool) {
	limit := telemetry.HistoryDefaultPageLimit
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 || parsed > telemetry.HistoryMaximumPageLimit {
			s.writeHistoryError(w, r, telemetry.HistoryReasonFilterInvalid)
			return 0, historyCursor{}, false
		}
		limit = parsed
	}
	cursor := historyCursor{Generation: data.snapshot.Generation, Dataset: options.dataset, QueryHash: historyQueryHash(r.URL.Query(), options)}
	if value := r.URL.Query().Get("cursor"); value != "" {
		parsed, err := decodeHistoryCursor(value)
		if err != nil || parsed.Generation != cursor.Generation || parsed.Dataset != cursor.Dataset || parsed.QueryHash != cursor.QueryHash || parsed.Offset < 0 || parsed.Offset > length {
			s.writeHistoryError(w, r, telemetry.HistoryReasonCursorInvalid)
			return 0, historyCursor{}, false
		}
		cursor = parsed
	}
	return limit, cursor, true
}

func historyQueryHash(query url.Values, options historyQueryOptions) string {
	clone := url.Values{}
	for key, values := range query {
		if key != "cursor" && key != "limit" {
			clone[key] = append([]string(nil), values...)
		}
	}
	clone.Set("sort", options.sort)
	clone.Set("dataset", options.dataset)
	hash := sha256.Sum256([]byte(clone.Encode()))
	return hex.EncodeToString(hash[:8])
}

func encodeHistoryCursor(cursor historyCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeHistoryCursor(value string) (historyCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return historyCursor{}, err
	}
	var cursor historyCursor
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.QueryHash == "" {
		return historyCursor{}, fmt.Errorf("invalid cursor")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return historyCursor{}, fmt.Errorf("invalid cursor")
	}
	return cursor, nil
}

func (s *Server) writeHistoryBackendError(w http.ResponseWriter, r *http.Request, err error) {
	code := telemetry.HistoryErrorCode(err)
	var commandErr *commandError
	if errors.As(err, &commandErr) {
		code = telemetry.HistoryReasonCode(commandErr.code)
	}
	s.writeHistoryError(w, r, code)
}

func (s *Server) writeHistoryError(w http.ResponseWriter, r *http.Request, code telemetry.HistoryReasonCode) {
	status := http.StatusBadRequest
	switch code {
	case telemetry.HistoryReasonRunNotFound:
		status = http.StatusNotFound
	case telemetry.HistoryReasonUnavailable:
		status = http.StatusServiceUnavailable
	}
	s.writeError(w, status, string(code), requestIDFrom(r), nil)
}

func writeHistoryRunCSV(writer *csv.Writer, runs []telemetry.HistoryRunAnalysis) {
	_ = writer.Write(telemetry.HistoryRunCSVColumns())
	for _, run := range runs {
		_ = writer.Write(sanitizeCSVRow([]string{
			run.RunID, run.StartedAt.UTC().Format(time.RFC3339Nano), run.Character, run.Difficulty, run.Run, run.RouteID,
			string(run.Outcome), run.Reason, strconv.FormatInt(run.DurationMs, 10), strconv.Itoa(run.BossKills),
			strconv.Itoa(run.Funnel.KeepReturn), strconv.Itoa(run.Funnel.Sold), strconv.Itoa(run.Funnel.PickupLost), strconv.Itoa(run.Funnel.PostPickupLost),
		}))
	}
}

func writeHistoryItemCSV(writer *csv.Writer, items []telemetry.HistoryItemAggregate) {
	_ = writer.Write(telemetry.HistoryItemCSVColumns())
	for _, item := range items {
		_ = writer.Write(sanitizeCSVRow([]string{
			item.ItemKey, item.ItemName, strconv.Itoa(item.Seen), strconv.Itoa(item.Matched), strconv.Itoa(item.PickedUp),
			strconv.Itoa(item.Stashed), strconv.Itoa(item.Sold), strconv.Itoa(item.PickupLost), strconv.Itoa(item.PostPickupLost),
			formatOptionalFloat(item.YieldPerRun), formatOptionalFloat(item.YieldPerKill), formatOptionalFloat(item.YieldPerHour),
		}))
	}
}

func sanitizeCSVRow(row []string) []string {
	out := make([]string, len(row))
	for index, value := range row {
		trimmed := strings.TrimLeft(value, " \t\r\n")
		leading := strings.TrimLeft(value, " \r\n")
		if (trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0]))) || strings.HasPrefix(leading, "\t") {
			value = "'" + value
		}
		out[index] = value
	}
	return out
}

func formatOptionalFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'g', -1, 64)
}
