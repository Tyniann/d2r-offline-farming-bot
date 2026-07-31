package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestPreviewPickitSocketMatrix(t *testing.T) {
	backend := newPickitAPIBackend(t)
	server := startPickitAPIServer(t, backend)

	profile := PickitProfileDTO{
		SchemaVersion: 1, Revision: 1, ID: "socket-preview", Name: "Socket Preview",
		Rules: []PickitRuleDTO{{ID: "four", Action: "keep", Expression: `[type] == "pole" && [tier] == "elite" && [sockets] == 4`}},
	}

	positive := postPickitPreview(t, server, profile, PickitPreviewItemDTO{
		Code: "7s8", Type: "pole", Quality: "normal", BaseTier: "elite",
		Sockets: 4, SocketsAvailable: true, Socketed: true,
	})
	if !positive.Matched || positive.RuleID != "four" {
		t.Fatalf("positive preview = %+v", positive)
	}

	wrongSockets := postPickitPreview(t, server, profile, PickitPreviewItemDTO{
		Code: "7s8", Type: "pole", Quality: "normal", BaseTier: "elite",
		Sockets: 3, SocketsAvailable: true, Socketed: true,
	})
	if wrongSockets.Matched {
		t.Fatalf("wrong socket count matched: %+v", wrongSockets)
	}

	unavailable := postPickitPreview(t, server, profile, PickitPreviewItemDTO{
		Code: "7s8", Type: "pole", Quality: "normal", BaseTier: "elite",
		Sockets: 4, SocketsAvailable: false, Socketed: true,
	})
	if unavailable.Matched {
		t.Fatalf("unavailable fixture matched: %+v", unavailable)
	}

	neq := PickitProfileDTO{
		SchemaVersion: 1, Revision: 1, ID: "socket-neq", Name: "Socket Neq",
		Rules: []PickitRuleDTO{{ID: "not", Action: "keep", Expression: `[flag] != socketed`}},
	}
	neqUnavailable := postPickitPreview(t, server, neq, PickitPreviewItemDTO{
		Code: "xpl", Quality: "unique", SocketsAvailable: false,
	})
	if neqUnavailable.Matched {
		t.Fatalf("unavailable != socketed matched: %+v", neqUnavailable)
	}

	body, _ := json.Marshal(PickitPreviewRequest{
		Profile: profile,
		Item: PickitPreviewItemDTO{
			Code: "7s8", Type: "pole", Quality: "normal",
			SocketsAvailable: true, Socketed: true, Sockets: 0,
		},
	})
	response, err := http.Post(server.URL()+"/api/v1/pickit/preview", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		t.Fatal("contradictory socket fixture was accepted")
	}
}

func postPickitPreview(t *testing.T, server *Server, profile PickitProfileDTO, item PickitPreviewItemDTO) PickitPreviewDTO {
	t.Helper()
	body, err := json.Marshal(PickitPreviewRequest{Profile: profile, Item: item})
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(server.URL()+"/api/v1/pickit/preview", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var raw map[string]any
		_ = json.NewDecoder(response.Body).Decode(&raw)
		t.Fatalf("preview status=%d body=%v", response.StatusCode, raw)
	}
	var dto PickitPreviewDTO
	if err := json.NewDecoder(response.Body).Decode(&dto); err != nil {
		t.Fatal(err)
	}
	return dto
}
