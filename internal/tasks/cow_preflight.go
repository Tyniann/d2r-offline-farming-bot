package tasks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const cowPreflightStableSnapshots = 3

// Cow preflight terminal reasons are stable queue-facing contracts.
const (
	CowReasonScopeUnsupported        = "cow_scope_unsupported"
	CowReasonCubeMissing             = "cow_cube_missing"
	CowReasonCubeAmbiguous           = "cow_cube_ambiguous"
	CowReasonCubeUnprotected         = "cow_cube_unprotected"
	CowReasonCubeNotEmpty            = "cow_cube_not_empty"
	CowReasonExistingLeg             = "cow_existing_leg"
	CowReasonInventorySpaceMissing   = "cow_inventory_space_missing"
	CowReasonReturnPortalUnavailable = "cow_return_portal_unavailable"
	CowReasonCombatSkillMissing      = "cow_combat_skill_missing"
	CowReasonCapabilityMissing       = "run_capability_missing"
)

// CowConfig freezes the strict operator scope and runtime capabilities for one
// Cow execution generation.
type CowConfig struct {
	Character           string
	ExpectedClass       world.CharacterClass
	ExpectedClassKnown  bool
	Difficulty          string
	ClientWidth         int
	ClientHeight        int
	InventoryLocked     [4][10]bool
	HasTownPortal       bool
	HasTeleport         bool
	RequiredSkillsReady bool
	HasTownServices     bool
}

type cowPreflight struct {
	config         CowConfig
	lastGeneration uint64
	lastFinding    string
	lastSignature  string
	stable         int
}

func (p *cowPreflight) reset() {
	p.lastGeneration = 0
	p.lastFinding = ""
	p.lastSignature = ""
	p.stable = 0
}

func (p *cowPreflight) tick(state world.State, setupRouteID, sweepRouteID string, windowWidth, windowHeight int) (done bool, reason string) {
	if !state.Valid || state.Phase != world.GamePhaseInGame || state.Generation == 0 || state.Generation == p.lastGeneration {
		return false, ""
	}
	p.lastGeneration = state.Generation
	finding, signature := evaluateCowPreflight(p.config, state, setupRouteID, sweepRouteID, windowWidth, windowHeight)
	if finding != p.lastFinding || signature != p.lastSignature {
		p.lastFinding, p.lastSignature, p.stable = finding, signature, 1
		return false, ""
	}
	p.stable++
	if p.stable < cowPreflightStableSnapshots {
		return false, ""
	}
	return true, finding
}

func evaluateCowPreflight(cfg CowConfig, state world.State, setupRouteID, sweepRouteID string, windowWidth, windowHeight int) (string, string) {
	if !state.Identity.Valid || !strings.EqualFold(state.Identity.CharacterName, cfg.Character) ||
		!cfg.ExpectedClassKnown || state.Identity.Class != cfg.ExpectedClass || !strings.EqualFold(cfg.Difficulty, "hell") ||
		windowWidth != cfg.ClientWidth || windowHeight != cfg.ClientHeight || state.Area.ID != world.RogueEncampment {
		return CowReasonScopeUnsupported, cowPreflightSignature(state)
	}
	if strings.TrimSpace(setupRouteID) == "" || strings.TrimSpace(sweepRouteID) == "" || !cfg.HasTownServices {
		return CowReasonCapabilityMissing, cowPreflightSignature(state)
	}

	cubes := make([]world.Item, 0, 1)
	for _, item := range state.InventoryItems() {
		if item.Code == "box" {
			cubes = append(cubes, item)
		}
	}
	if len(cubes) == 0 {
		return CowReasonCubeMissing, cowPreflightSignature(state)
	}
	if len(cubes) != 1 || cubes[0].Width != 2 || cubes[0].Height != 2 || !validInventoryFootprint(cubes[0]) {
		return CowReasonCubeAmbiguous, cowPreflightSignature(state)
	}
	for row := cubes[0].GridY; row < cubes[0].GridY+cubes[0].Height; row++ {
		for col := cubes[0].GridX; col < cubes[0].GridX+cubes[0].Width; col++ {
			if !cfg.InventoryLocked[row][col] {
				return CowReasonCubeUnprotected, cowPreflightSignature(state)
			}
		}
	}
	if len(state.ItemsByLocation(world.ItemLocationCube)) != 0 {
		return CowReasonCubeNotEmpty, cowPreflightSignature(state)
	}
	for _, item := range state.Items {
		if item.Code == "leg" && visibleCowItemLocation(item.Location) {
			return CowReasonExistingLeg, cowPreflightSignature(state)
		}
	}
	if !cowInventoryCanFitBoth(cfg.InventoryLocked, state.InventoryItems()) {
		return CowReasonInventorySpaceMissing, cowPreflightSignature(state)
	}
	hasOperationalTome := false
	for _, item := range state.InventoryItems() {
		if item.Code == "tbk" {
			hasOperationalTome = true
			break
		}
	}
	if !cfg.HasTownPortal || !hasOperationalTome {
		return CowReasonReturnPortalUnavailable, cowPreflightSignature(state)
	}
	if !cfg.HasTeleport || !cfg.RequiredSkillsReady {
		return CowReasonCombatSkillMissing, cowPreflightSignature(state)
	}
	return "", cowPreflightSignature(state)
}

func visibleCowItemLocation(location world.ItemLocation) bool {
	switch location {
	case world.ItemLocationInventory, world.ItemLocationCube, world.ItemLocationStash,
		world.ItemLocationSharedStash1, world.ItemLocationSharedStash2, world.ItemLocationSharedStash3:
		return true
	default:
		return false
	}
}

func validInventoryFootprint(item world.Item) bool {
	return item.GridX >= 0 && item.GridY >= 0 && item.Width > 0 && item.Height > 0 &&
		item.GridX+item.Width <= 10 && item.GridY+item.Height <= 4
}

// CowInventoryCanFitRecipeItems reports whether unlocked personal inventory can
// hold both a 1×3 Wirt's Leg and a 1×2 Town-Portal tome at the same time.
func CowInventoryCanFitRecipeItems(locked [4][10]bool, items []world.Item) bool {
	return cowInventoryCanFitBoth(locked, items)
}

func cowInventoryCanFitBoth(locked [4][10]bool, items []world.Item) bool {
	occupied := [4][10]bool{}
	for _, item := range items {
		if !validInventoryFootprint(item) {
			return false
		}
		for row := item.GridY; row < item.GridY+item.Height; row++ {
			for col := item.GridX; col < item.GridX+item.Width; col++ {
				if occupied[row][col] {
					return false
				}
				occupied[row][col] = true
			}
		}
	}
	for legRow := 0; legRow <= 1; legRow++ {
		for legCol := 0; legCol < 10; legCol++ {
			if !cowRectangleFree(locked, occupied, legCol, legRow, 1, 3) {
				continue
			}
			withLeg := occupied
			cowMarkRectangle(&withLeg, legCol, legRow, 1, 3)
			for tomeRow := 0; tomeRow <= 2; tomeRow++ {
				for tomeCol := 0; tomeCol < 10; tomeCol++ {
					if cowRectangleFree(locked, withLeg, tomeCol, tomeRow, 1, 2) {
						return true
					}
				}
			}
		}
	}
	return false
}

func cowRectangleFree(locked, occupied [4][10]bool, col, row, width, height int) bool {
	for y := row; y < row+height; y++ {
		for x := col; x < col+width; x++ {
			if locked[y][x] || occupied[y][x] {
				return false
			}
		}
	}
	return true
}

func cowMarkRectangle(occupied *[4][10]bool, col, row, width, height int) {
	for y := row; y < row+height; y++ {
		for x := col; x < col+width; x++ {
			occupied[y][x] = true
		}
	}
}

func cowPreflightSignature(state world.State) string {
	items := make([]string, 0, len(state.Items))
	for _, item := range state.Items {
		items = append(items, fmt.Sprintf("%d:%s:%s:%d:%d:%d:%d", item.UnitID, item.Code, item.Location, item.GridX, item.GridY, item.Width, item.Height))
	}
	sort.Strings(items)
	return strings.Join(items, "|")
}
