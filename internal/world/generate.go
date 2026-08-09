package world

// Keep every source version explicit: generated catalogs must never silently
// combine local TXT exports from another D2R build.
//go:generate go run ../../tools/generate-item-catalog -src ../../.tmp/d2r-excel -version 3.2.92777 -out item_catalog_data.go
//go:generate go run ../../tools/generate-monster-catalog -src ../../.tmp/d2r-excel -version 3.2.92777 -memory-out ../memory/monster_ids_data.go -world-out monster_ids_data.go
//go:generate go run ../../tools/generate-object-catalog -src ../../.tmp/d2r-excel -version 3.2.92777 -memory-out ../memory/object_ids_data.go -world-out object_ids_data.go
