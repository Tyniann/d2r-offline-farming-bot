package town

import (
	"fmt"
	"strings"
)

// HubConfig describes the only full Town service graph: Rogue Encampment.
type HubConfig struct {
	Area            string             `yaml:"area"`
	Anchors         []Anchor           `yaml:"anchors"`
	Services        map[Service]Anchor `yaml:"services"`
	RoutesDirectory string             `yaml:"routes_directory"`
}

// EgressConfig permits a foreign act to register only its local departure route.
type EgressConfig struct {
	Area            string             `yaml:"area"`
	Anchors         []Anchor           `yaml:"anchors"`
	RoutesDirectory string             `yaml:"routes_directory"`
	Services        map[Service]Anchor `yaml:"services"`
}

// Config contains the central hub and the minimal foreign-act egress registry.
type Config struct {
	Thresholds Thresholds                 `yaml:"thresholds"`
	Hub        HubConfig                  `yaml:"hub"`
	Egress     map[OriginAct]EgressConfig `yaml:"egress"`
	present    bool
}

// UnmarshalYAML records whether the optional Town section was supplied.
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	type alias Config
	var decoded alias
	if err := unmarshal(&decoded); err != nil {
		return err
	}
	*c = Config(decoded)
	c.present = true
	return nil
}

// Validate rejects incomplete hubs, ambiguous anchors, unsupported providers, and invalid egresses.
func (c Config) Validate() error {
	if !c.present {
		return nil
	}
	for name, value := range map[string]int{"healing": c.Thresholds.Healing, "mana": c.Thresholds.Mana, "town_portal_scrolls": c.Thresholds.TownPortalScrolls, "identify_scrolls": c.Thresholds.IdentifyScrolls} {
		if value < 0 {
			return fmt.Errorf("town.thresholds.%s must be >= 0", name)
		}
	}
	if c.Hub.Area != "rogue_encampment" {
		return fmt.Errorf("town.hub.area must be rogue_encampment")
	}
	if strings.TrimSpace(c.Hub.RoutesDirectory) == "" {
		return fmt.Errorf("town.hub.routes_directory is required")
	}
	seen := map[Anchor]bool{}
	for _, anchor := range c.Hub.Anchors {
		if !hubAnchor(anchor) {
			return fmt.Errorf("town.hub.anchors contains unknown anchor %q", anchor)
		}
		if seen[anchor] {
			return fmt.Errorf("town.hub.anchors contains duplicate anchor %q", anchor)
		}
		seen[anchor] = true
	}
	for _, required := range []Anchor{AnchorSpawn, AnchorPortalArrival, AnchorStash, AnchorWaypoint, AnchorAkara, AnchorCharsi, AnchorCain} {
		if !seen[required] {
			return fmt.Errorf("town.hub.anchors missing %q", required)
		}
	}
	for service, vendor := range map[Service]Anchor{ServicePotions: AnchorAkara, ServiceScrolls: AnchorAkara, ServiceIdentify: AnchorCain, ServiceSell: AnchorAkara, ServiceRepair: AnchorCharsi} {
		if c.Hub.Services[service] != vendor {
			return fmt.Errorf("town.hub.services.%s must be %s", service, vendor)
		}
	}
	for act, egress := range c.Egress {
		if act != OriginAct2 && act != OriginAct3 && act != OriginAct4 && act != OriginAct5 {
			return fmt.Errorf("town.egress.%s is unsupported", act)
		}
		if strings.TrimSpace(egress.Area) == "" || strings.TrimSpace(egress.RoutesDirectory) == "" {
			return fmt.Errorf("town.egress.%s area and routes_directory are required", act)
		}
		if len(egress.Anchors) != 2 || egress.Anchors[0] != AnchorPortalArrival || egress.Anchors[1] != AnchorWaypoint {
			return fmt.Errorf("town.egress.%s anchors must be portal_arrival, waypoint", act)
		}
		if len(egress.Services) != 0 {
			return fmt.Errorf("town.egress.%s must not define services", act)
		}
	}
	return nil
}

// EgressFor resolves the foreign-act egress or returns the stable fail-closed reason.
func (c Config) EgressFor(act OriginAct) (EgressConfig, Reason) {
	egress, ok := c.Egress[act]
	if !ok {
		return EgressConfig{}, ReasonEgressMissing
	}
	return egress, ""
}

func hubAnchor(anchor Anchor) bool {
	switch anchor {
	case AnchorSpawn, AnchorPortalArrival, AnchorStash, AnchorWaypoint, AnchorAkara, AnchorCharsi, AnchorCain:
		return true
	}
	return false
}
