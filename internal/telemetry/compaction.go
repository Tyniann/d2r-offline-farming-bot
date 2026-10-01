package telemetry

// CompactionProgress beschreibt einen bestätigten Rezeptzyklus innerhalb eines Auftrags.
type CompactionProgress struct {
	RecipeKey    string `json:"recipe_key"`
	RecipeIndex  int    `json:"recipe_index"`
	SourceCode   string `json:"source_code"`
	OutputCode   string `json:"output_code"`
	SourceBefore int    `json:"source_before"`
	SourceAfter  int    `json:"source_after"`
	OutputBefore int    `json:"output_before"`
	OutputAfter  int    `json:"output_after"`
}

// CompactedMaterial zählt bestätigte, rückgelagerte Rezeptresultate je Zielcode.
// Später weiterverdichtete Zwischenstufen bleiben als geleistete Arbeit gezählt.
type CompactedMaterial struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}
