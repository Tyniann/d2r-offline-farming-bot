package world

import "strings"

// ItemIdentityKind unterscheidet die numerisch überlappenden Set- und Unique-ID-Räume.
type ItemIdentityKind string

const (
	// ItemIdentitySet bezeichnet eine Set-Item-Identität.
	ItemIdentitySet ItemIdentityKind = "set"
	// ItemIdentityUnique bezeichnet eine Unique-Item-Identität.
	ItemIdentityUnique ItemIdentityKind = "unique"
)

// ItemIdentityCatalogEntry ist ein build-time erzeugter, patchgenauer Identitätseintrag.
type ItemIdentityCatalogEntry struct {
	Kind        ItemIdentityKind
	RawID       uint32
	Key         string
	DisplayName string
	BaseCode    string
	SetKey      string
	SetName     string
	Spawnable   bool
}

// ItemIdentityCatalogVersion liefert die D2R-Quellversion des eingebetteten Katalogs.
func ItemIdentityCatalogVersion() string {
	return generatedItemIdentityCatalogVersion
}

// ItemIdentityCatalogEntries liefert eine defensive Kopie aller echten Set-/Unique-Zeilen.
func ItemIdentityCatalogEntries() []ItemIdentityCatalogEntry {
	return append([]ItemIdentityCatalogEntry(nil), generatedItemIdentityCatalog...)
}

// LookupItemIdentity löst eine rohe Set-/Unique-ID innerhalb ihres getrennten ID-Raums auf.
func LookupItemIdentity(kind ItemIdentityKind, rawID uint32) (ItemIdentityCatalogEntry, bool) {
	for _, entry := range generatedItemIdentityCatalog {
		if entry.Kind == kind && entry.RawID == rawID {
			return entry, true
		}
	}
	return ItemIdentityCatalogEntry{}, false
}

// LookupItemIdentityKey löst einen stabilen Katalogschlüssel case-insensitiv eindeutig auf.
func LookupItemIdentityKey(kind ItemIdentityKind, key string) (ItemIdentityCatalogEntry, bool) {
	for _, entry := range generatedItemIdentityCatalog {
		if entry.Kind == kind && strings.EqualFold(entry.Key, key) {
			return entry, true
		}
	}
	return ItemIdentityCatalogEntry{}, false
}
