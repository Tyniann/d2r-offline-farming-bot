package memory

import (
	"encoding/binary"
	"testing"
)

func TestCollectKnownSkillsCompleteNilTerminatedList(t *testing.T) {
	access := newMockAccess()
	reader := newTestReader(access)
	reader.Bind(access)
	probe := NewProbeReader(reader, testOffsetSet())

	const (
		listPtr    = 0x40000
		skillA     = 0x41000
		skillB     = 0x42000
		txtA       = 0x43000
		txtB       = 0x44000
		leftSkill  = 0x45000
		rightSkill = 0x46000
		leftTxt    = 0x47000
		rightTxt   = 0x48000
	)
	writeSkillNode(access, skillA, txtA, SkillTeleport, skillB)
	writeSkillNode(access, skillB, txtB, SkillBoneSpear, 0)
	writeSkillNode(access, leftSkill, leftTxt, SkillBoneArmor, 0)
	writeSkillNode(access, rightSkill, rightTxt, SkillTeleport, 0)
	list := make([]byte, 0x18)
	binary.LittleEndian.PutUint64(list[0:], uint64(skillA))
	binary.LittleEndian.PutUint64(list[0x08:], uint64(leftSkill))
	binary.LittleEndian.PutUint64(list[0x10:], uint64(rightSkill))
	access.setBytes(listPtr, list)
	unit := make([]byte, 0x108)
	binary.LittleEndian.PutUint64(unit[0x100:], uint64(listPtr))
	access.setBytes(0x1000, unit)
	off := testOffsetSet()
	off.Unit.SkillsList = 0x100

	got, err := probe.readPlayerSkills(0x1000, off)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Complete || got.IncompleteReason != "" {
		t.Fatalf("complete=%v reason=%q", got.Complete, got.IncompleteReason)
	}
	if !got.HasSkill(SkillTeleport) || !got.HasSkill(SkillBoneSpear) || got.HasSkill(SkillTownPortal) {
		t.Fatalf("known=%v", got.SkillsKnown)
	}
	if got.LeftSkill != SkillBoneArmor || got.RightSkill != SkillTeleport {
		t.Fatalf("mouse skills=%d/%d", got.LeftSkill, got.RightSkill)
	}
}

func TestCollectKnownSkillsEmptyHeadIsComplete(t *testing.T) {
	access := newMockAccess()
	reader := newTestReader(access)
	reader.Bind(access)
	probe := NewProbeReader(reader, testOffsetSet())

	const listPtr = 0x40000
	leftSkill, rightSkill := uintptr(0x45000), uintptr(0x46000)
	leftTxt, rightTxt := uintptr(0x47000), uintptr(0x48000)
	writeSkillNode(access, leftSkill, leftTxt, SkillBoneArmor, 0)
	writeSkillNode(access, rightSkill, rightTxt, SkillTeleport, 0)
	list := make([]byte, 0x18)
	binary.LittleEndian.PutUint64(list[0x08:], uint64(leftSkill))
	binary.LittleEndian.PutUint64(list[0x10:], uint64(rightSkill))
	access.setBytes(listPtr, list)
	unit := make([]byte, 0x108)
	binary.LittleEndian.PutUint64(unit[0x100:], uint64(listPtr))
	access.setBytes(0x1000, unit)
	off := testOffsetSet()
	off.Unit.SkillsList = 0x100

	got, err := probe.readPlayerSkills(0x1000, off)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Complete || len(got.SkillsKnown) != 0 {
		t.Fatalf("got=%+v", got)
	}
	if got.HasSkill(SkillTeleport) {
		t.Fatal("incomplete-safe HasSkill must stay false for empty complete list")
	}
}

func TestCollectKnownSkillsReadFailureIsIncomplete(t *testing.T) {
	access := newMockAccess()
	reader := newTestReader(access)
	reader.Bind(access)
	probe := NewProbeReader(reader, testOffsetSet())

	const (
		listPtr = 0x40000
		skillA  = 0x41000
		txtA    = 0x43000
	)
	leftSkill, rightSkill := uintptr(0x45000), uintptr(0x46000)
	leftTxt, rightTxt := uintptr(0x47000), uintptr(0x48000)
	writeSkillNode(access, skillA, txtA, SkillTeleport, 0xDEAD0000) // next points to unread memory
	writeSkillNode(access, leftSkill, leftTxt, SkillBoneArmor, 0)
	writeSkillNode(access, rightSkill, rightTxt, SkillTeleport, 0)
	list := make([]byte, 0x18)
	binary.LittleEndian.PutUint64(list[0:], uint64(skillA))
	binary.LittleEndian.PutUint64(list[0x08:], uint64(leftSkill))
	binary.LittleEndian.PutUint64(list[0x10:], uint64(rightSkill))
	access.setBytes(listPtr, list)
	unit := make([]byte, 0x108)
	binary.LittleEndian.PutUint64(unit[0x100:], uint64(listPtr))
	access.setBytes(0x1000, unit)
	off := testOffsetSet()
	off.Unit.SkillsList = 0x100

	got, err := probe.readPlayerSkills(0x1000, off)
	if err != nil {
		t.Fatal(err)
	}
	if got.Complete || got.IncompleteReason != SkillListIncompleteRead {
		t.Fatalf("got=%+v", got)
	}
	if got.HasSkill(SkillTeleport) {
		t.Fatal("incomplete list must not authorize HasSkill")
	}
	if !got.SkillsKnown[SkillTeleport] {
		t.Fatal("partial known IDs should still be retained for diagnostics")
	}
}

func TestCollectKnownSkillsLimitExceeded(t *testing.T) {
	access := newMockAccess()
	reader := newTestReader(access)
	reader.Bind(access)
	probe := NewProbeReader(reader, testOffsetSet())

	const listPtr = 0x40000
	leftSkill, rightSkill := uintptr(0xF0000), uintptr(0xF1000)
	leftTxt, rightTxt := uintptr(0xF2000), uintptr(0xF3000)
	writeSkillNode(access, leftSkill, leftTxt, SkillBoneArmor, 0)
	writeSkillNode(access, rightSkill, rightTxt, SkillTeleport, 0)

	first := uintptr(0x50000)
	for i := 0; i <= MaxPlayerSkillListNodes; i++ {
		node := first + uintptr(i)*0x20
		txt := first + uintptr(MaxPlayerSkillListNodes+2)*0x20 + uintptr(i)*0x10
		next := first + uintptr(i+1)*0x20
		writeSkillNode(access, node, txt, SkillTeleport, next)
	}
	list := make([]byte, 0x18)
	binary.LittleEndian.PutUint64(list[0:], uint64(first))
	binary.LittleEndian.PutUint64(list[0x08:], uint64(leftSkill))
	binary.LittleEndian.PutUint64(list[0x10:], uint64(rightSkill))
	access.setBytes(listPtr, list)
	unit := make([]byte, 0x108)
	binary.LittleEndian.PutUint64(unit[0x100:], uint64(listPtr))
	access.setBytes(0x1000, unit)
	off := testOffsetSet()
	off.Unit.SkillsList = 0x100

	got, err := probe.readPlayerSkills(0x1000, off)
	if err != nil {
		t.Fatal(err)
	}
	if got.Complete || got.IncompleteReason != SkillListLimitExceeded {
		t.Fatalf("got=%+v", got)
	}
}

func writeSkillNode(access *mockAccess, node, txt uintptr, skillID uint16, next uintptr) {
	nodeBuf := make([]byte, 0x10)
	binary.LittleEndian.PutUint64(nodeBuf[0:], uint64(txt))
	binary.LittleEndian.PutUint64(nodeBuf[0x08:], uint64(next))
	access.setBytes(node, nodeBuf)
	txtBuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(txtBuf, skillID)
	access.setBytes(txt, txtBuf)
}
