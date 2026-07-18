package town

import "testing"

func validTownConfig() Config {
	return Config{present: true, Hub: HubConfig{Area: "rogue_encampment", RoutesDirectory: "routes/town/act1/graph", Anchors: []Anchor{AnchorSpawn, AnchorPortalArrival, AnchorStash, AnchorWaypoint, AnchorAkara, AnchorCharsi, AnchorCain}, Services: map[Service]Anchor{ServicePotions: AnchorAkara, ServiceScrolls: AnchorAkara, ServiceIdentify: AnchorCain, ServiceSell: AnchorAkara, ServiceRepair: AnchorCharsi}}, Egress: map[OriginAct]EgressConfig{OriginAct3: {Area: "kurast_docks", Anchors: []Anchor{AnchorPortalArrival, AnchorWaypoint}, RoutesDirectory: "routes/town/act3/egress"}}}
}

func TestTownConfigValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"missing hub", func(c *Config) { c.Hub.Area = "" }},
		{"duplicate anchor", func(c *Config) { c.Hub.Anchors = append(c.Hub.Anchors, AnchorStash) }},
		{"unknown provider", func(c *Config) { c.Hub.Services[ServicePotions] = AnchorCharsi }},
		{"service in egress", func(c *Config) {
			e := c.Egress[OriginAct3]
			e.Services = map[Service]Anchor{ServiceSell: AnchorAkara}
			c.Egress[OriginAct3] = e
		}},
		{"foreign missing egress", func(c *Config) { c.Egress = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validTownConfig()
			tc.mutate(&cfg)
			if tc.name == "foreign missing egress" {
				if _, reason := cfg.EgressFor(OriginAct3); reason != ReasonEgressMissing {
					t.Fatalf("reason = %q", reason)
				}
				return
			}
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate accepted invalid config")
			}
		})
	}
}
