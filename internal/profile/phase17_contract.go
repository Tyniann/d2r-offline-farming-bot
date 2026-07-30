package profile

// ResourceContext ergänzt die bestehende Resource Policy um den schmalen
// Route-Kontext. Die vorhandenen [Result]-Statuswerte bleiben die einzige
// Autorität dafür, ob ein Tick Input gesendet hat oder passiv verifiziert.
type ResourceContext struct {
	// MobilityCritical ist wahr, wenn die Teleport-Manareserve gehalten werden muss.
	MobilityCritical bool
	// Threatened ist wahr, wenn das aktuelle immutable Assessment eine Bedrohung enthält.
	Threatened bool
	// EmergencyMana ist ausschließlich bei Immediate-Threat und kritischem Mana wahr.
	EmergencyMana bool
}
