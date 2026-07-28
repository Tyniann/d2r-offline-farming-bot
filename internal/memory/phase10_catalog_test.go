package memory

import "testing"

func TestGeneratedMephistoIsRuntimeMonsterCandidate(t *testing.T) {
	for _, id := range []uint32{242, 250, 526} {
		if !IsRuntimeMonsterCandidate(id, 0) {
			t.Fatalf("boss *hcIdx %d is absent from the generated runtime allowlist", id)
		}
	}
	if IsRuntimeMonsterCandidate(241, 0) {
		t.Fatal("unregistered normal monster unexpectedly passed the runtime allowlist")
	}
}
