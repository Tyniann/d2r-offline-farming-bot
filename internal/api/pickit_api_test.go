package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

func newPickitAPIBackend(t *testing.T) *LiveBackend {
	t.Helper()
	root := t.TempDir()
	profiles, err := app.NewPickitProfileService(root + "/profiles")
	if err != nil {
		t.Fatal(err)
	}
	if mkdirErr := fsMkdir(root + "/profiles"); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	_, err = profiles.Create(app.PickitProfileDocument{SchemaVersion: 1, Revision: 1, ID: "base", Name: "Base", Rules: []app.PickitProfileRuleDocument{{ID: "rune", Action: loot.ActionKeep, Expression: `[type] == "rune"`}}})
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := app.NewPickitAssignmentStore(root+"/assignments.yaml", profiles)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = assignments.Initialize(map[string]map[tasks.RunID][]string{"MrBones": {tasks.RunIDCountess: {"base"}}}); err != nil {
		t.Fatal(err)
	}
	return &LiveBackend{publisher: telemetry.NewLivePublisher(16, 4), status: StatusDTO{State: string(app.SupervisorStateIdle)}, routeWorkflow: RouteWorkflowDTO{State: string(app.RouteWorkflowIdle)}, pickitProfiles: profiles, pickitAssignments: assignments}
}

func fsMkdir(path string) error { return os.MkdirAll(path, 0o755) }

func startPickitAPIServer(t *testing.T, backend *LiveBackend) *Server {
	t.Helper()
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok"), Mode: fs.FileMode(0o644)}}
	server, err := New(Config{Backend: backend, Assets: assets, Events: backend.publisher})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})
	return server
}

func TestPickitAPIContractAndUnauthorizedMutation(t *testing.T) {
	backend := newPickitAPIBackend(t)
	server := startPickitAPIServer(t, backend)
	response, err := http.Get(server.URL() + "/api/v1/pickit/catalog")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var catalog PickitCatalogDTO
	if decodeErr := json.NewDecoder(response.Body).Decode(&catalog); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if response.StatusCode != http.StatusOK || catalog.CatalogVersion == "" || len(catalog.Bases) == 0 || len(catalog.Identities) == 0 {
		t.Fatalf("catalog status=%d value=%+v", response.StatusCode, catalog)
	}
	body := `{"profile":{"schema_version":1,"revision":1,"id":"new-profile","name":"Neu","rules":[{"id":"rule","action":"keep","expression":"[type] == \"rune\""}]}}`
	request, _ := http.NewRequest(http.MethodPost, server.URL()+"/api/v1/pickit/profiles", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", response.StatusCode)
	}
	request, _ = http.NewRequest(http.MethodPost, server.URL()+"/api/v1/pickit/profiles", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(controlTokenHeader, server.token)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status=%d", response.StatusCode)
	}
}

func TestPickitParallelSavesStaleRevisionAndRunLock(t *testing.T) {
	backend := newPickitAPIBackend(t)
	base := PickitProfileDTO{SchemaVersion: 1, Revision: 1, ID: "base", Name: "A", Rules: []PickitRuleDTO{{ID: "rune", Action: "keep", Expression: `[type] == "rune"`}}}
	requests := []PickitUpdateRequest{{ExpectedRevision: 1, Profile: base}, {ExpectedRevision: 1, Profile: base}}
	requests[0].Profile.Name, requests[1].Profile.Name = "Erste", "Zweite"
	var wg sync.WaitGroup
	errorsFound := make(chan error, 2)
	for _, request := range requests {
		wg.Add(1)
		go func(value PickitUpdateRequest) {
			defer wg.Done()
			_, err := backend.UpdatePickit("base", value)
			errorsFound <- err
		}(request)
	}
	wg.Wait()
	close(errorsFound)
	successes, conflicts := 0, 0
	for err := range errorsFound {
		if err == nil {
			successes++
		} else if strings.Contains(err.Error(), "revision_conflict") {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("parallel saves successes=%d conflicts=%d", successes, conflicts)
	}
	current, _ := backend.pickitProfiles.Get("base")
	_, err := backend.UpdatePickit("base", PickitUpdateRequest{ExpectedRevision: 1, Profile: PickitProfileDTO{SchemaVersion: 1, Revision: current.Revision, ID: "base", Name: "Stale", Rules: base.Rules}})
	if err == nil || !strings.Contains(err.Error(), "revision_conflict") {
		t.Fatalf("stale update err=%v", err)
	}
	backend.mu.Lock()
	backend.status.State = string(app.SupervisorStateRunningRun)
	backend.mu.Unlock()
	_, err = backend.UpdatePickit("base", PickitUpdateRequest{ExpectedRevision: current.Revision, Profile: profileDTO(current)})
	var commandErr *commandError
	if !errors.As(err, &commandErr) || commandErr.code != "command_conflict" {
		t.Fatalf("run lock err=%v", err)
	}
}
