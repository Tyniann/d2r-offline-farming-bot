package memory

import "testing"

func TestCaptureUIBufferCopiesDiagnosticWindow(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()
	uiBase := moduleBase + off.UI - uiBufferBefore
	want := make([]byte, uiBufferSize)
	want[0] = 7
	want[uiGateIndex] = 1
	want[uiLoadingIndex] = 2
	access.setBytes(uiBase, want)

	capture, err := probe.CaptureUIBuffer()
	if err != nil {
		t.Fatalf("CaptureUIBuffer() error = %v", err)
	}
	if capture.Anchor != uiBufferBefore || len(capture.Bytes) != uiBufferSize {
		t.Fatalf("capture shape = anchor %d len %d", capture.Anchor, len(capture.Bytes))
	}
	if capture.Bytes[0] != 7 || capture.Bytes[uiGateIndex] != 1 || capture.Bytes[uiLoadingIndex] != 2 {
		t.Fatalf("capture bytes did not preserve diagnostic window")
	}
	if capture.OffsetFromAnchor(uiGateIndex) != uiGateIndex-uiBufferBefore {
		t.Fatalf("relative gate offset = %d", capture.OffsetFromAnchor(uiGateIndex))
	}

	want[0] = 9
	access.setBytes(uiBase, want)
	if capture.Bytes[0] != 7 {
		t.Fatal("capture must own an immutable copy")
	}
}

func TestCaptureUIBufferRequiresAttachedReader(t *testing.T) {
	probe := NewProbeReader(nil, testOffsetSet())
	if _, err := probe.CaptureUIBuffer(); err == nil {
		t.Fatal("expected unattached capture error")
	}
}
