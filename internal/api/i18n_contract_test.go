package api

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestOpenAPIErrorContractIsLanguageNeutral(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("schema/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Components struct {
			Schemas map[string]struct {
				Required   []string                   `json:"required"`
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}

	errorSchema := document.Components.Schemas["ErrorDTO"]
	if !reflect.DeepEqual(errorSchema.Required, []string{"code", "request_id"}) {
		t.Fatalf("ErrorDTO required fields = %v", errorSchema.Required)
	}
	assertSchemaProperties(t, errorSchema.Properties, []string{"code", "params", "request_id"})
	problemSchema := document.Components.Schemas["ProblemDTO"]
	if !reflect.DeepEqual(problemSchema.Required, []string{"code"}) {
		t.Fatalf("ProblemDTO required fields = %v", problemSchema.Required)
	}
	assertSchemaProperties(t, problemSchema.Properties, []string{"code", "params"})

	var languageFields, requiredMessages []string
	for schemaName, schema := range document.Components.Schemas {
		for property := range schema.Properties {
			if strings.HasSuffix(property, "_de") || strings.HasSuffix(property, "_en") {
				languageFields = append(languageFields, schemaName+"."+property)
			}
		}
		for _, required := range schema.Required {
			if required == "message" {
				requiredMessages = append(requiredMessages, schemaName+"."+required)
			}
		}
	}
	sort.Strings(languageFields)
	sort.Strings(requiredMessages)
	if len(languageFields) != 0 {
		t.Fatalf("language-specific OpenAPI fields = %v, want none", languageFields)
	}
	if len(requiredMessages) != 0 {
		t.Fatalf("required OpenAPI message fields = %v, want none", requiredMessages)
	}
	for _, forbidden := range []string{"instructions_de", "operator_hints_de", "reason_message"} {
		if strings.Contains(string(data), `"`+forbidden+`"`) {
			t.Fatalf("OpenAPI still contains forbidden transport field %q", forbidden)
		}
	}
	runProgress := document.Components.Schemas["RunProgressDTO"]
	if _, exists := runProgress.Properties["label"]; exists {
		t.Fatal("RunProgressDTO still exposes localized label")
	}
}

func assertSchemaProperties(t *testing.T, properties map[string]json.RawMessage, want []string) {
	t.Helper()
	got := make([]string, 0, len(properties))
	for property := range properties {
		got = append(got, property)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schema properties = %v, want %v", got, want)
	}
}
