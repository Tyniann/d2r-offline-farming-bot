package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplayRealOfflineMephistoHardStuckFixture(t *testing.T) {
	path := filepath.Join("testdata", "mephisto-live-hard-stuck.trace.gz")
	bundle, err := ReadBundle(path, 1<<20)
	if err != nil {
		t.Fatalf("ReadBundle() error = %v", err)
	}
	report, err := Replay(bundle)
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if report.Ticks != 328 || report.Step != "play_bound_route" || report.Outcome != "failed" || report.Reason != "hard_stuck" {
		t.Fatalf("Replay() report = %+v", report)
	}
	if len(bundle.Checkpoints) != 0 || bundle.Contract.Character != "" || len(bundle.Contract.Loadout) != 0 {
		t.Fatalf("live fixture retains non-decision identity payload: contract=%+v checkpoints=%d", bundle.Contract, len(bundle.Checkpoints))
	}
	for _, frame := range bundle.Frames {
		if len(frame.World.Items) != 0 || len(frame.World.Monsters) != 0 || len(frame.World.Objects) != 0 || len(frame.World.Entrances) != 0 || len(frame.World.CowCorpses) != 0 {
			t.Fatalf("tick %d retains entities proven irrelevant to this failure", frame.Tick)
		}
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"MrBones", "C:\\Users\\", "D:\\CSharpProjekte\\", "mephisto-mrbones-"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("live fixture contains local identity %q", forbidden)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 32<<10 {
		t.Fatalf("minimized live fixture size = %d bytes, want <= 32 KiB", info.Size())
	}
}
