package memory

// Keep every source version explicit: generated catalogs must never silently
// combine local TXT exports from another D2R build.
//
//go:generate go run ../../tools/generate-skill-catalog -src ../../.tmp/d2r-excel -version 3.2.92777 -out skill_catalog_data.go
