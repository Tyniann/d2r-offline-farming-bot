package town

import "github.com/Tyniann/d2r-offline-farming-bot/internal/world"

// MercenaryPolicy is the effective combat-profile Merc support used by Town.
type MercenaryPolicy struct {
	Enabled          bool
	ThresholdPercent int
}

// EvaluateMercenaryTownDemand derives mutually exclusive heal/revive demand or
// a terminal Town reason from one coherent Merc snapshot. Disabled support
// yields no Merc service. Dead always wins over heal.
func EvaluateMercenaryTownDemand(policy MercenaryPolicy, merc world.Mercenary) (heal, revive bool, fail Reason) {
	if !policy.Enabled {
		return false, false, ""
	}
	threshold := policy.ThresholdPercent
	if threshold <= 0 {
		threshold = 50
	}
	if !merc.HiredKnown {
		return false, false, ReasonMercenaryStateInvalid
	}
	if !merc.Hired {
		return false, false, ReasonMercenaryNotHired
	}
	if merc.Dead {
		if merc.Alive {
			return false, false, ReasonMercenaryStateInvalid
		}
		return false, true, ""
	}
	if !merc.Alive {
		return false, false, ReasonMercenaryStateInvalid
	}
	if !merc.VitalsKnown {
		return false, false, ReasonMercenaryStateInvalid
	}
	if int(merc.HPPercent()) < threshold {
		return true, false, ""
	}
	return false, false, ""
}

// EvaluateMercenaryPreflight rejects session start when Merc support is enabled
// but the hireling is missing, already dead, or unreadable. Injured living Mercs
// are allowed; Town heal runs only after a farming run.
func EvaluateMercenaryPreflight(policy MercenaryPolicy, merc world.Mercenary) Reason {
	if !policy.Enabled {
		return ""
	}
	if !merc.HiredKnown {
		return ReasonMercenaryStateInvalid
	}
	if !merc.Hired {
		return ReasonMercenaryNotHired
	}
	if merc.Dead {
		if merc.Alive {
			return ReasonMercenaryStateInvalid
		}
		return ReasonMercenaryDeadAtStart
	}
	if !merc.Alive || !merc.VitalsKnown {
		return ReasonMercenaryStateInvalid
	}
	return ""
}
