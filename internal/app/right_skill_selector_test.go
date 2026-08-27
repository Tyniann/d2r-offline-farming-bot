package app

import (
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestMissingRequiredSkillsIgnoresIncompleteLists(t *testing.T) {
	player := world.Player{SkillsKnown: map[uint16]bool{}, SkillsComplete: false}
	missing := missingRequiredSkills(player, []uint16{memory.SkillTeleport}, map[uint16]string{memory.SkillTeleport: "Teleport"})
	if len(missing) != 0 {
		t.Fatalf("incomplete list produced missing=%v", missing)
	}
}

func TestMissingRequiredSkillsListsGermanLabels(t *testing.T) {
	player := world.Player{
		SkillsComplete: true,
		SkillsKnown:    map[uint16]bool{memory.SkillTeleport: true},
	}
	missing := missingRequiredSkills(player, []uint16{memory.SkillTeleport, memory.SkillBoneSpear, memory.SkillTownPortal}, map[uint16]string{
		memory.SkillBoneSpear:  "Knochen-Speer",
		memory.SkillTownPortal: "Stadtportal",
	})
	if len(missing) != 2 || missing[0] != "Knochen-Speer" || missing[1] != "Stadtportal" {
		t.Fatalf("missing=%v", missing)
	}
}

func TestMissingRequiredSkillsAcceptsTownPortalBookEvidence(t *testing.T) {
	player := world.Player{
		SkillsComplete: true,
		SkillsKnown: map[uint16]bool{
			memory.SkillTeleport:                     true,
			memory.MustSkillID("book_of_townportal"): true,
		},
	}
	missing := missingRequiredSkills(
		player,
		[]uint16{memory.SkillTeleport, memory.SkillTownPortal},
		map[uint16]string{memory.SkillTownPortal: "Stadtportal"},
	)
	if len(missing) != 0 {
		t.Fatalf("town portal book evidence produced missing=%v", missing)
	}
}

func TestMissingRequiredSkillsAcceptsLooseTownPortalScrollEvidence(t *testing.T) {
	player := world.Player{
		SkillsComplete: true,
		SkillsKnown: map[uint16]bool{
			memory.SkillTeleport:                       true,
			memory.MustSkillID("scroll_of_townportal"): true,
		},
	}
	missing := missingRequiredSkills(
		player,
		[]uint16{memory.SkillTeleport, memory.SkillTownPortal},
		map[uint16]string{memory.SkillTownPortal: "Stadtportal"},
	)
	if len(missing) != 0 {
		t.Fatalf("town portal scroll evidence produced missing=%v", missing)
	}
}

func TestMissingRequiredSkillsAcceptsSlingTownPortalEvidence(t *testing.T) {
	player := world.Player{
		SkillsComplete: true,
		SkillsKnown: map[uint16]bool{
			memory.SkillTeleport:                     true,
			memory.MustSkillID("townportal_o_skill"): true,
		},
	}
	missing := missingRequiredSkills(
		player,
		[]uint16{memory.SkillTeleport, memory.SkillTownPortal},
		map[uint16]string{memory.SkillTownPortal: "Stadtportal"},
	)
	if len(missing) != 0 {
		t.Fatalf("Sling Town Portal evidence produced missing=%v", missing)
	}
}

func TestRightSkillSelectorAcceptsSlingTownPortalAsSelected(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTownPortal: {SkillID: memory.SkillTownPortal, SelectKey: "f6", CastButton: input.MouseRight},
	}}
	selector := NewRightSkillSelector(bindings, in)
	slingTP := memory.MustSkillID("townportal_o_skill")
	clicked := false
	sent, err := selector.EnsureAndCast(memory.SkillTownPortal, slingTP, time.Now(), func() error {
		clicked = true
		return nil
	})
	if err != nil || !sent || !clicked || in.selectCalls != 0 {
		t.Fatalf("sling already-selected cast sent=%v err=%v clicked=%v selects=%d", sent, err, clicked, in.selectCalls)
	}
}

func TestRightSkillSelectorConfirmsSlingTownPortalAfterSelect(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTownPortal: {SkillID: memory.SkillTownPortal, SelectKey: "f6", CastButton: input.MouseRight},
	}}
	selector := NewRightSkillSelector(bindings, in)
	now := time.Now()
	sent, err := selector.EnsureAndCast(memory.SkillTownPortal, memory.SkillBoneSpear, now, nil)
	if err != nil || sent || in.selectCalls != 1 {
		t.Fatalf("select phase sent=%v err=%v selects=%d", sent, err, in.selectCalls)
	}
	slingTP := memory.MustSkillID("townportal_o_skill")
	clicked := false
	sent, err = selector.EnsureAndCast(memory.SkillTownPortal, slingTP, now.Add(10*time.Millisecond), func() error {
		clicked = true
		return nil
	})
	if err != nil || !sent || !clicked {
		t.Fatalf("confirm sling TP sent=%v err=%v clicked=%v", sent, err, clicked)
	}
}

func TestRightSkillSelectorCastsImmediatelyWhenAlreadySelected(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	selector := NewRightSkillSelector(bindings, in)
	clicked := false
	sent, err := selector.EnsureAndCast(memory.SkillTeleport, memory.SkillTeleport, time.Now(), func() error {
		clicked = true
		return nil
	})
	if err != nil || !sent || !clicked || in.selectCalls != 0 {
		t.Fatalf("sent=%t err=%v clicked=%t selects=%d", sent, err, clicked, in.selectCalls)
	}
}

func TestRightSkillSelectorWaitsThenConfirms(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	selector := NewRightSkillSelector(bindings, in)
	now := time.Now()
	sent, err := selector.EnsureAndCast(memory.SkillTeleport, memory.SkillBoneSpear, now, func() error {
		t.Fatal("click during select")
		return nil
	})
	if err != nil || sent || in.selectCalls != 1 {
		t.Fatalf("select sent=%t err=%v selects=%d", sent, err, in.selectCalls)
	}
	sent, err = selector.EnsureAndCast(memory.SkillTeleport, memory.SkillBoneSpear, now.Add(10*time.Millisecond), func() error {
		t.Fatal("click while unchanged")
		return nil
	})
	if err != nil || sent {
		t.Fatalf("wait sent=%t err=%v", sent, err)
	}
	clicks := 0
	sent, err = selector.EnsureAndCast(memory.SkillTeleport, memory.SkillTeleport, now.Add(20*time.Millisecond), func() error {
		clicks++
		return nil
	})
	if err != nil || !sent || clicks != 1 || in.selectCalls != 1 {
		t.Fatalf("confirm sent=%t err=%v clicks=%d selects=%d", sent, err, clicks, in.selectCalls)
	}
}

func TestRightSkillSelectorDoesNotPreemptPendingSelection(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillAmplifyDamage: {SkillID: memory.SkillAmplifyDamage, SelectKey: "f1", CastButton: input.MouseRight},
		memory.SkillTeleport:      {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	selector := NewRightSkillSelector(bindings, in)
	now := time.Now()
	sent, err := selector.EnsureAndCast(memory.SkillAmplifyDamage, memory.SkillTeleport, now, nil)
	if err != nil || sent || in.selectCalls != 1 || selector.selector.pending[SkillSlotRight] != memory.SkillAmplifyDamage {
		t.Fatalf("AD select sent=%v err=%v selects=%d pending=%d", sent, err, in.selectCalls, selector.selector.pending[SkillSlotRight])
	}

	sent, err = selector.EnsureAndCast(memory.SkillTeleport, memory.SkillTeleport, now.Add(10*time.Millisecond), func() error {
		t.Fatal("teleport must not cast while AD selection is pending")
		return nil
	})
	if err != nil || sent || in.selectCalls != 1 || selector.selector.pending[SkillSlotRight] != memory.SkillAmplifyDamage {
		t.Fatalf("teleport preempt sent=%v err=%v selects=%d pending=%d", sent, err, in.selectCalls, selector.selector.pending[SkillSlotRight])
	}

	clicks := 0
	sent, err = selector.EnsureAndCast(memory.SkillAmplifyDamage, memory.SkillAmplifyDamage, now.Add(20*time.Millisecond), func() error {
		clicks++
		return nil
	})
	if err != nil || !sent || clicks != 1 || selector.selector.pending[SkillSlotRight] != 0 {
		t.Fatalf("AD confirm sent=%v err=%v clicks=%d pending=%d", sent, err, clicks, selector.selector.pending[SkillSlotRight])
	}

	sent, err = selector.EnsureAndCast(memory.SkillTeleport, memory.SkillAmplifyDamage, now.Add(30*time.Millisecond), nil)
	if err != nil || sent || in.selectCalls != 2 || selector.selector.pending[SkillSlotRight] != memory.SkillTeleport {
		t.Fatalf("teleport after AD sent=%v err=%v selects=%d pending=%d", sent, err, in.selectCalls, selector.selector.pending[SkillSlotRight])
	}
}

func TestRightSkillSelectorTimesOutAndRejectsMismatch(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	selector := NewRightSkillSelector(bindings, in)
	now := time.Now()
	if _, err := selector.EnsureAndCast(memory.SkillTeleport, memory.SkillBoneSpear, now, nil); err != nil {
		t.Fatal(err)
	}
	sent, err := selector.EnsureAndCast(memory.SkillTeleport, memory.SkillBoneSpear, now.Add(rightSkillSelectionTimeout), func() error {
		t.Fatal("click after timeout")
		return nil
	})
	if sent || err == nil || !strings.Contains(err.Error(), "right_skill_selection_unconfirmed") {
		t.Fatalf("timeout sent=%t err=%v", sent, err)
	}
	if selector.selector.pending[SkillSlotRight] != 0 {
		t.Fatal("pending must clear after timeout")
	}

	if _, ensureErr := selector.EnsureAndCast(memory.SkillTeleport, memory.SkillBoneSpear, now.Add(2*time.Second), nil); ensureErr != nil {
		t.Fatal(ensureErr)
	}
	sent, err = selector.EnsureAndCast(memory.SkillTeleport, memory.SkillTownPortal, now.Add(2*time.Second+10*time.Millisecond), func() error {
		t.Fatal("click on mismatch")
		return nil
	})
	if sent || err == nil || !strings.Contains(err.Error(), "right_skill_selection_unconfirmed") {
		t.Fatalf("mismatch sent=%t err=%v", sent, err)
	}
}

func TestRightSkillSelectorRejectsLeftButtonAndResetClearsPending(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseLeft},
	}}
	selector := NewRightSkillSelector(bindings, in)
	sent, err := selector.EnsureAndCast(memory.SkillTeleport, memory.SkillBoneSpear, time.Now(), nil)
	if sent || err == nil || !strings.Contains(err.Error(), "must use right mouse") {
		t.Fatalf("left button sent=%t err=%v", sent, err)
	}

	bindings.skills[memory.SkillTeleport] = input.SkillCast{SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight}
	selector = NewRightSkillSelector(bindings, in)
	if _, err := selector.EnsureAndCast(memory.SkillTeleport, memory.SkillBoneSpear, time.Now(), nil); err != nil {
		t.Fatal(err)
	}
	selector.Reset()
	if selector.selector.pending[SkillSlotRight] != 0 {
		t.Fatal("reset must clear pending")
	}
}

func TestSkillSelectorConfirmsLeftSkillOnlyOnNewerTick(t *testing.T) {
	in := &recordingCombatInput{}
	hammer := memory.MustSkillID("blessed_hammer")
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		hammer: {SkillID: hammer, SelectKey: "f1", CastButton: input.MouseLeft},
	}}
	selector := NewSkillSelector(bindings, in)
	now := time.Now()
	if sent, err := selector.EnsureAndCast(SkillSlotLeft, hammer, 0, 0, now, nil); err != nil || sent || in.selectCalls != 1 {
		t.Fatalf("select sent=%v err=%v calls=%d", sent, err, in.selectCalls)
	}
	if sent, err := selector.EnsureAndCast(SkillSlotLeft, hammer, hammer, 0, now, func() error {
		t.Fatal("same tick must not cast")
		return nil
	}); err != nil || sent {
		t.Fatalf("same-tick confirmation sent=%v err=%v", sent, err)
	}
	casts := 0
	if sent, err := selector.EnsureAndCast(SkillSlotLeft, hammer, hammer, 0, now.Add(time.Millisecond), func() error {
		casts++
		return nil
	}); err != nil || !sent || casts != 1 {
		t.Fatalf("fresh confirmation sent=%v err=%v casts=%d", sent, err, casts)
	}
}

func TestSkillSelectorKeepsLeftAndRightPendingIndependent(t *testing.T) {
	in := &recordingCombatInput{}
	hammer := memory.MustSkillID("blessed_hammer")
	teleport := memory.SkillTeleport
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		hammer:   {SkillID: hammer, SelectKey: "f1", CastButton: input.MouseLeft},
		teleport: {SkillID: teleport, SelectKey: "f2", CastButton: input.MouseRight},
	}}
	selector := NewSkillSelector(bindings, in)
	now := time.Now()
	if _, err := selector.EnsureAndCast(SkillSlotLeft, hammer, 0, 0, now, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := selector.EnsureAndCast(SkillSlotRight, teleport, 0, memory.SkillBoneSpear, now, nil); err != nil {
		t.Fatal(err)
	}
	if selector.pending[SkillSlotLeft] != hammer || selector.pending[SkillSlotRight] != teleport || in.selectCalls != 2 {
		t.Fatalf("pending=%v selectCalls=%d", selector.pending, in.selectCalls)
	}
}

func TestProfileRequiredSkillsMissingResultListsLabels(t *testing.T) {
	result := profileRequiredSkillsMissingResult("MrBones", "Knochen-Speer", []string{"Knochenrüstung", "Stadtportal"})
	if result.Disposition != QueueRunStop || result.Reason != reasonProfileRequiredSkillsMissing || result.ExitAuthorization != ExitAuthorizationMemoryGatedCurrentArea {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(result.Detail, "MrBones fehlen für Knochen-Speer") || !strings.Contains(result.Detail, "Knochenrüstung") {
		t.Fatalf("detail = %q", result.Detail)
	}
}

func TestResolveRequiredSkillIDsUsesCatalog(t *testing.T) {
	ids, labels, err := resolveRequiredSkillIDs([]config.RequiredSkillConfig{
		{Skill: "teleport", DisplayName: "Teleport"},
		{Skill: "bone_spear", DisplayName: "Knochen-Speer"},
	})
	if err != nil || len(ids) != 2 || ids[0] != memory.SkillTeleport || labels[memory.SkillBoneSpear] != "Knochen-Speer" {
		t.Fatalf("ids=%v labels=%v err=%v", ids, labels, err)
	}
}
