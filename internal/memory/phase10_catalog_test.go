package memory

import "testing"

func TestGeneratedMephistoIsRuntimeMonsterCandidate(t *testing.T) {
	if !IsRuntimeMonsterCandidate(242, 0) {
		t.Fatal("Mephisto *hcIdx 242 is absent from the generated runtime allowlist")
	}
	if IsRuntimeMonsterCandidate(241, 0) {
		t.Fatal("unregistered normal monster unexpectedly passed the runtime allowlist")
	}
}
