package memory

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

// CollectionCount is a count on a collection proxy, independent of ItemUnit.Quantity.
// TxtFileNo binds the value to the local CASC item catalog in World.
type CollectionCount struct {
	TxtFileNo uint32
	UnitID    uint32
	Count     uint32
}

// CollectionSnapshot is a complete, twice-read collection inventory for one
// generation. ScopeID identifies only its live process instance, never a saved
// character or a persistent shared-storage namespace. The zero value is unknown.
type CollectionSnapshot struct {
	ScopeID    string
	Available  bool
	Generation uint64
	Counts     []CollectionCount
	Tab        string
	TabKnown   bool
	Reason     string
}

const (
	// Gate 25.1, 2026-10-01, D2R 3.2.92777: gsr/glr/gpr proxy counts
	// 22/97/5 matched the existing Gems screenshot. Parent ownership, all
	// 75 links and both inventory endpoints were verified in raw research.
	collectionDataSize   = 0xB8
	collectionCountAt    = 0x9C
	collectionParentAt   = 0xA0
	collectionPreviousAt = 0xA8
	collectionNextAt     = 0xB0
	collectionHeaderSize = 0x50
	collectionSignature  = 0x01020304
	collectionOwnerAt    = 0x08
	collectionFirstAt    = 0x10
	collectionLastAt     = 0x18
	collectionLengthAt   = 0x4C
)

// readCollectionStorage deliberately discovers the parent through its owner
// inventory, not through the fail-open item enumeration. A missing node, cycle,
// ambiguous parent or changing second read revokes the entire count source.
// UI tab and embedded Cube state are not inferred from proxy presence.
func (p *ProbeReader) readCollectionStorage(moduleBase uintptr, off OffsetSet, snap Snapshot) CollectionSnapshot {
	if !snap.Valid || snap.Phase != GamePhaseInGame || !snap.UI.StashOpen || snap.Generation == 0 || off.D2RVersion != "3.2.92777" {
		return CollectionSnapshot{}
	}
	if access, ok := p.reader.access.(interface{ Status() process.Status }); ok {
		status := access.Status()
		if status.State != process.StateAttached || status.FileVersion != "3.2.92777" {
			return CollectionSnapshot{Reason: "collection_process_version_unavailable"}
		}
	}
	root, err := p.findCollectionInventory(moduleBase, off)
	if err != nil {
		return CollectionSnapshot{Reason: err.Error()}
	}
	tabBefore, tabErr := p.readCollectionTab(moduleBase)
	first, header, err := p.readCollectionInventory(root, off)
	if err != nil {
		return CollectionSnapshot{Reason: err.Error()}
	}
	second, again, err := p.readCollectionInventory(root, off)
	if err != nil || !bytes.Equal(header, again) || !slices.Equal(first, second) {
		return CollectionSnapshot{Reason: "collection_changed_or_unreadable"}
	}
	tabAfter, tabAfterErr := p.readCollectionTab(moduleBase)
	_, _, loading, currentUI := p.readPhaseInputs(moduleBase, off)
	if loading || currentUI != snap.UI {
		return CollectionSnapshot{Reason: "collection_ui_changed_or_unreadable"}
	}
	// Replay carries an opaque equality key, never process-memory addresses.
	// Game/process reset still belongs to the runtime lifecycle; this hash is
	// not a promise of storage identity across detach or a new game.
	var scope [24]byte
	binary.LittleEndian.PutUint64(scope[:], uint64(moduleBase))
	binary.LittleEndian.PutUint64(scope[8:], uint64(root))
	binary.LittleEndian.PutUint64(scope[16:], binary.LittleEndian.Uint64(header[collectionOwnerAt:]))
	digest := sha256.Sum256(scope[:])
	result := CollectionSnapshot{ScopeID: hex.EncodeToString(digest[:16]), Available: true, Generation: snap.Generation, Counts: first}
	// Tab reads enclose both count reads. Unknown/changing UI does not invent a
	// tab or revoke independently proven counts; action guards require TabKnown.
	if tabErr == nil && tabAfterErr == nil && tabBefore == tabAfter {
		result.Tab, result.TabKnown = tabAfter.tab, true
	}
	return result
}

func (p *ProbeReader) findCollectionInventory(moduleBase uintptr, off OffsetSet) (uintptr, error) {
	heads, err := p.reader.ReadBytes(unitSegmentBase(moduleBase, off.UnitTable, unitSegmentPlayer), unitTableSegmentBytes)
	if err != nil {
		return 0, fmt.Errorf("collection owner table: %w", err)
	}
	seen := make(map[uintptr]bool)
	var root uintptr
	for bucket := 0; bucket < unitTableListHeads; bucket++ {
		unit := uintptr(binary.LittleEndian.Uint64(heads[bucket*unitTableHeadStride:]))
		for unit != 0 {
			if seen[unit] || len(seen) >= maxTotalUnitVisits {
				return 0, fmt.Errorf("collection owner walk incomplete")
			}
			seen[unit] = true
			kind, e := p.reader.ReadUint32(unit + off.Unit.UnitType)
			if e != nil || kind != 0 {
				return 0, fmt.Errorf("collection owner unit unreadable or invalid")
			}
			inventory, e := p.reader.ReadUint64(unit + off.Unit.Inventory)
			if e != nil {
				return 0, fmt.Errorf("collection owner inventory: %w", e)
			}
			if inventory != 0 {
				first, firstErr := p.reader.ReadUint64(uintptr(inventory) + collectionFirstAt)
				if firstErr != nil {
					return 0, fmt.Errorf("collection first item: %w", firstErr)
				}
				if first != 0 {
					proxy, proxyErr := p.isCollectionProxy(uintptr(first), off)
					if proxyErr != nil {
						return 0, proxyErr
					}
					if proxy {
						if root != 0 {
							return 0, fmt.Errorf("collection inventory ambiguous")
						}
						root = uintptr(inventory)
					}
				}
			}
			next, e := p.reader.ReadUint64(unit + off.Unit.NextUnit)
			if e != nil {
				return 0, fmt.Errorf("collection next owner: %w", e)
			}
			unit = uintptr(next)
		}
	}
	if root == 0 {
		return 0, fmt.Errorf("collection inventory unavailable")
	}
	return root, nil
}

func (p *ProbeReader) isCollectionProxy(unit uintptr, off OffsetSet) (bool, error) {
	kind, err := p.reader.ReadUint32(unit + off.Unit.UnitType)
	if err != nil || kind != itemUnitType {
		return false, fmt.Errorf("collection proxy unit unreadable or invalid")
	}
	location, err := p.reader.ReadUint32(unit + itemOffsetRawLocation)
	if err != nil {
		return false, fmt.Errorf("collection proxy location: %w", err)
	}
	data, err := p.reader.ReadUint64(unit + off.Unit.UnitData)
	if err != nil || data == 0 {
		return false, fmt.Errorf("collection proxy data unavailable")
	}
	owner, err := p.reader.ReadUint32(uintptr(data) + itemDataOffsetOwnerID)
	if err != nil {
		return false, fmt.Errorf("collection proxy owner: %w", err)
	}
	page, err := p.reader.ReadUint8(uintptr(data) + itemDataOffsetPage)
	if err != nil {
		return false, fmt.Errorf("collection proxy page: %w", err)
	}
	return location == itemRawLocationInventory && owner == ^uint32(0) && page == 4, nil
}

func (p *ProbeReader) readCollectionInventory(root uintptr, off OffsetSet) ([]CollectionCount, []byte, error) {
	header, err := p.reader.ReadBytes(root, collectionHeaderSize)
	if err != nil {
		return nil, nil, fmt.Errorf("collection header: %w", err)
	}
	n := binary.LittleEndian.Uint32(header[collectionLengthAt:])
	owner := uintptr(binary.LittleEndian.Uint64(header[collectionOwnerAt:]))
	first := uintptr(binary.LittleEndian.Uint64(header[collectionFirstAt:]))
	last := uintptr(binary.LittleEndian.Uint64(header[collectionLastAt:]))
	if binary.LittleEndian.Uint32(header) != collectionSignature || owner == 0 || first == 0 || last == 0 || n == 0 || n > maxItemsPerSnapshot {
		return nil, nil, fmt.Errorf("collection header invalid")
	}
	kind, kindErr := p.reader.ReadUint32(owner + off.Unit.UnitType)
	back, backErr := p.reader.ReadUint64(owner + off.Unit.Inventory)
	if kindErr != nil || backErr != nil || kind != 0 || uintptr(back) != root {
		return nil, nil, fmt.Errorf("collection owner backreference invalid")
	}
	counts := make([]CollectionCount, 0, n)
	seen := make(map[uintptr]bool)
	ids := make(map[uint32]bool)
	var previous uintptr
	for unit := first; unit != 0; {
		if seen[unit] || uint32(len(counts)) >= n {
			return nil, nil, fmt.Errorf("collection item cycle or length mismatch")
		}
		seen[unit] = true
		proxy, e := p.isCollectionProxy(unit, off)
		if e != nil || !proxy {
			return nil, nil, fmt.Errorf("collection item is not a readable proxy")
		}
		id, idErr := p.reader.ReadUint32(unit + off.Unit.UnitID)
		txt, txtErr := p.reader.ReadUint32(unit + unitOffsetTxtFileNo)
		dataPtr, ptrErr := p.reader.ReadUint64(unit + off.Unit.UnitData)
		if idErr != nil || txtErr != nil || ptrErr != nil || id == 0 || ids[id] {
			return nil, nil, fmt.Errorf("collection item identity invalid")
		}
		ids[id] = true
		data, e := p.reader.ReadBytes(uintptr(dataPtr), collectionDataSize)
		if e != nil {
			return nil, nil, fmt.Errorf("collection item data: %w", e)
		}
		if uintptr(binary.LittleEndian.Uint64(data[collectionParentAt:])) != root || uintptr(binary.LittleEndian.Uint64(data[collectionPreviousAt:])) != previous {
			return nil, nil, fmt.Errorf("collection item backreference invalid")
		}
		count := binary.LittleEndian.Uint32(data[collectionCountAt:])
		if count > 99 {
			return nil, nil, fmt.Errorf("collection count out of range")
		}
		counts = append(counts, CollectionCount{TxtFileNo: txt, UnitID: id, Count: count})
		previous = unit
		unit = uintptr(binary.LittleEndian.Uint64(data[collectionNextAt:]))
	}
	if uint32(len(counts)) != n || previous != last {
		return nil, nil, fmt.Errorf("collection item endpoints or length mismatch")
	}
	return counts, header, nil
}
