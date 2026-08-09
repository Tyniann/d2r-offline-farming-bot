// Area IDs and display names predate the local catalog generators. Phase-20
// identities StonyField, Tristram, and MooMooFarm are authoritatively verified
// against D2R 3.2.92777 local data/global/excel/levels.txt. The remaining legacy
// names cover IDs 0..136; constants extend through 141.
package world

// Area ID constants; Phase-20 entries use the local levels.txt contract above.
const (
	Abaddon                  AreaID = 125
	AncientTunnels           AreaID = 65
	ArcaneSanctuary          AreaID = 74
	ArreatPlateau            AreaID = 112
	ArreatSummit             AreaID = 120
	Barracks                 AreaID = 28
	BlackMarsh               AreaID = 6
	BloodMoor                AreaID = 2
	BloodyFoothills          AreaID = 110
	BurialGrounds            AreaID = 17
	CanyonOfTheMagi          AreaID = 46
	CatacombsLevel1          AreaID = 34
	CatacombsLevel2          AreaID = 35
	CatacombsLevel3          AreaID = 36
	CatacombsLevel4          AreaID = 37
	Cathedral                AreaID = 33
	CaveLevel1               AreaID = 9
	CaveLevel2               AreaID = 13
	ChaosSanctuary           AreaID = 108
	CityOfTheDamned          AreaID = 106
	ClawViperTempleLevel1    AreaID = 58
	ClawViperTempleLevel2    AreaID = 61
	ColdPlains               AreaID = 3
	Crypt                    AreaID = 18
	CrystallinePassage       AreaID = 113
	DarkWood                 AreaID = 5
	DenOfEvil                AreaID = 8
	DisusedFane              AreaID = 95
	DisusedReliquary         AreaID = 99
	DrifterCavern            AreaID = 116
	DryHills                 AreaID = 42
	DuranceOfHateLevel1      AreaID = 100
	DuranceOfHateLevel2      AreaID = 101
	DuranceOfHateLevel3      AreaID = 102
	DurielsLair              AreaID = 73
	FarOasis                 AreaID = 43
	FlayerDungeonLevel1      AreaID = 88
	FlayerDungeonLevel2      AreaID = 89
	FlayerDungeonLevel3      AreaID = 91
	FlayerJungle             AreaID = 78
	ForgottenReliquary       AreaID = 96
	ForgottenSands           AreaID = 134
	ForgottenTemple          AreaID = 97
	ForgottenTower           AreaID = 20
	FrigidHighlands          AreaID = 111
	FrozenRiver              AreaID = 114
	FrozenTundra             AreaID = 117
	FurnaceOfPain            AreaID = 135
	GlacialTrail             AreaID = 115
	GreatMarsh               AreaID = 77
	HallsOfAnguish           AreaID = 122
	HallsOfPain              AreaID = 123
	HallsOfTheDeadLevel1     AreaID = 56
	HallsOfTheDeadLevel2     AreaID = 57
	HallsOfTheDeadLevel3     AreaID = 60
	HallsOfVaught            AreaID = 124
	HaremLevel1              AreaID = 50
	HaremLevel2              AreaID = 51
	Harrogath                AreaID = 109
	HoleLevel1               AreaID = 11
	HoleLevel2               AreaID = 15
	IcyCellar                AreaID = 119
	InfernalPit              AreaID = 127
	InnerCloister            AreaID = 32
	JailLevel1               AreaID = 29
	JailLevel2               AreaID = 30
	JailLevel3               AreaID = 31
	KurastBazaar             AreaID = 80
	KurastCauseway           AreaID = 82
	KurastDocks              AreaID = 75
	LostCity                 AreaID = 44
	LowerKurast              AreaID = 79
	LutGholein               AreaID = 40
	MaggotLairLevel1         AreaID = 62
	MaggotLairLevel2         AreaID = 63
	MaggotLairLevel3         AreaID = 64
	MatronsDen               AreaID = 133
	Mausoleum                AreaID = 19
	MonasteryGate            AreaID = 26
	MooMooFarm               AreaID = 39
	NihlathaksTemple         AreaID = 121
	None                     AreaID = 0
	OuterCloister            AreaID = 27
	OuterSteppes             AreaID = 104
	PalaceCellarLevel1       AreaID = 52
	PalaceCellarLevel2       AreaID = 53
	PalaceCellarLevel3       AreaID = 54
	PitLevel1                AreaID = 12
	PitLevel2                AreaID = 16
	PitOfAcheron             AreaID = 126
	PlainsOfDespair          AreaID = 105
	RiverOfFlame             AreaID = 107
	RockyWaste               AreaID = 41
	RogueEncampment          AreaID = 1
	RuinedFane               AreaID = 98
	RuinedTemple             AreaID = 94
	SewersLevel1Act2         AreaID = 47
	SewersLevel1Act3         AreaID = 92
	SewersLevel2Act2         AreaID = 48
	SewersLevel2Act3         AreaID = 93
	SewersLevel3Act2         AreaID = 49
	SpiderCave               AreaID = 84
	SpiderCavern             AreaID = 85
	SpiderForest             AreaID = 76
	StonyField               AreaID = 4
	StonyTombLevel1          AreaID = 55
	StonyTombLevel2          AreaID = 59
	SwampyPitLevel1          AreaID = 86
	SwampyPitLevel2          AreaID = 87
	SwampyPitLevel3          AreaID = 90
	TalRashasTomb1           AreaID = 66
	TalRashasTomb2           AreaID = 67
	TalRashasTomb3           AreaID = 68
	TalRashasTomb4           AreaID = 69
	TalRashasTomb5           AreaID = 70
	TalRashasTomb6           AreaID = 71
	TalRashasTomb7           AreaID = 72
	TamoeHighland            AreaID = 7
	TheAncientsWay           AreaID = 118
	ThePandemoniumFortress   AreaID = 103
	TheWorldstoneChamber     AreaID = 132
	TheWorldStoneKeepLevel1  AreaID = 128
	TheWorldStoneKeepLevel2  AreaID = 129
	TheWorldStoneKeepLevel3  AreaID = 130
	ThroneOfDestruction      AreaID = 131
	TowerCellarLevel1        AreaID = 21
	TowerCellarLevel2        AreaID = 22
	TowerCellarLevel3        AreaID = 23
	TowerCellarLevel4        AreaID = 24
	TowerCellarLevel5        AreaID = 25
	Travincal                AreaID = 83
	Tristram                 AreaID = 38
	UberTristram             AreaID = 136
	UndergroundPassageLevel1 AreaID = 10
	UndergroundPassageLevel2 AreaID = 14
	UpperKurast              AreaID = 81
	ValleyOfSnakes           AreaID = 45
	MapsAncientTemple        AreaID = 137
	MapsDesecratedTemple     AreaID = 138
	MapsFrigidPlateau        AreaID = 139
	MapsInfernalTrial        AreaID = 140
	MapsRuinedCitadel        AreaID = 141
)

type areaEntry struct {
	name string
	kind AreaKind
}

// areaCatalog holds display names (IDs 1..136) and manually classified kinds.
// Act is never stored here; LookupArea sets it from AreaID.Act().
var areaCatalog = map[AreaID]areaEntry{
	1:   {name: "Rogue Encampment", kind: AreaKindUnknown},
	2:   {name: "Blood Moor", kind: AreaKindOutdoor},
	3:   {name: "Cold Plains", kind: AreaKindOutdoor},
	4:   {name: "Stony Field", kind: AreaKindOutdoor},
	5:   {name: "Dark Wood", kind: AreaKindOutdoor},
	6:   {name: "Black Marsh", kind: AreaKindOutdoor},
	7:   {name: "Tamoe Highland", kind: AreaKindUnknown},
	8:   {name: "Den of Evil", kind: AreaKindUnknown},
	9:   {name: "Cave Level 1", kind: AreaKindUnknown},
	10:  {name: "Underground Passage Level 1", kind: AreaKindUnknown},
	11:  {name: "Hole Level 1", kind: AreaKindUnknown},
	12:  {name: "Pit Level 1", kind: AreaKindUnknown},
	13:  {name: "Cave Level 2", kind: AreaKindUnknown},
	14:  {name: "Underground Passage Level 2", kind: AreaKindUnknown},
	15:  {name: "Hole Level 2", kind: AreaKindUnknown},
	16:  {name: "Pit Level 2", kind: AreaKindUnknown},
	17:  {name: "Burial Grounds", kind: AreaKindUnknown},
	18:  {name: "Crypt", kind: AreaKindUnknown},
	19:  {name: "Mausoleum", kind: AreaKindUnknown},
	20:  {name: "Forgotten Tower", kind: AreaKindSpecial},
	21:  {name: "Tower Cellar Level 1", kind: AreaKindDungeon},
	22:  {name: "Tower Cellar Level 2", kind: AreaKindDungeon},
	23:  {name: "Tower Cellar Level 3", kind: AreaKindDungeon},
	24:  {name: "Tower Cellar Level 4", kind: AreaKindDungeon},
	25:  {name: "Tower Cellar Level 5", kind: AreaKindDungeon},
	26:  {name: "Monastery Gate", kind: AreaKindUnknown},
	27:  {name: "Outer Cloister", kind: AreaKindUnknown},
	28:  {name: "Barracks", kind: AreaKindUnknown},
	29:  {name: "Jail Level 1", kind: AreaKindUnknown},
	30:  {name: "Jail Level 2", kind: AreaKindUnknown},
	31:  {name: "Jail Level 3", kind: AreaKindUnknown},
	32:  {name: "Inner Cloister", kind: AreaKindUnknown},
	33:  {name: "Cathedral", kind: AreaKindUnknown},
	34:  {name: "Catacombs Level 1", kind: AreaKindUnknown},
	35:  {name: "Catacombs Level 2", kind: AreaKindUnknown},
	36:  {name: "Catacombs Level 3", kind: AreaKindUnknown},
	37:  {name: "Catacombs Level 4", kind: AreaKindUnknown},
	38:  {name: "Tristram", kind: AreaKindUnknown},
	39:  {name: "Moo Moo Farm", kind: AreaKindUnknown},
	40:  {name: "Lut Gholein", kind: AreaKindUnknown},
	41:  {name: "Rocky Waste", kind: AreaKindUnknown},
	42:  {name: "Dry Hills", kind: AreaKindUnknown},
	43:  {name: "Far Oasis", kind: AreaKindUnknown},
	44:  {name: "Lost City", kind: AreaKindUnknown},
	45:  {name: "Valley of Snakes", kind: AreaKindUnknown},
	46:  {name: "Canyon of the Magi", kind: AreaKindUnknown},
	47:  {name: "Sewers Level 1", kind: AreaKindUnknown},
	48:  {name: "Sewers Level 2", kind: AreaKindUnknown},
	49:  {name: "Sewers Level 3", kind: AreaKindUnknown},
	50:  {name: "Harem Level 1", kind: AreaKindUnknown},
	51:  {name: "Harem Level 2", kind: AreaKindUnknown},
	52:  {name: "Palace Cellar Level 1", kind: AreaKindUnknown},
	53:  {name: "Palace Cellar Level 2", kind: AreaKindUnknown},
	54:  {name: "Palace Cellar Level 3", kind: AreaKindUnknown},
	55:  {name: "Stony Tomb Level 1", kind: AreaKindUnknown},
	56:  {name: "Halls of the Dead Level 1", kind: AreaKindUnknown},
	57:  {name: "Halls of the Dead Level 2", kind: AreaKindUnknown},
	58:  {name: "Claw Viper Temple Level 1", kind: AreaKindUnknown},
	59:  {name: "Stony Tomb Level 2", kind: AreaKindUnknown},
	60:  {name: "Halls of the Dead Level 3", kind: AreaKindUnknown},
	61:  {name: "Claw Viper Temple Level 2", kind: AreaKindUnknown},
	62:  {name: "Maggot Lair Level 1", kind: AreaKindUnknown},
	63:  {name: "Maggot Lair Level 2", kind: AreaKindUnknown},
	64:  {name: "Maggot Lair Level 3", kind: AreaKindUnknown},
	65:  {name: "Ancient Tunnels", kind: AreaKindUnknown},
	66:  {name: "Tal Rasha's Tomb", kind: AreaKindUnknown},
	67:  {name: "Tal Rasha's Tomb", kind: AreaKindUnknown},
	68:  {name: "Tal Rasha's Tomb", kind: AreaKindUnknown},
	69:  {name: "Tal Rasha's Tomb", kind: AreaKindUnknown},
	70:  {name: "Tal Rasha's Tomb", kind: AreaKindUnknown},
	71:  {name: "Tal Rasha's Tomb", kind: AreaKindUnknown},
	72:  {name: "Tal Rasha's Tomb", kind: AreaKindUnknown},
	73:  {name: "Duriel's Lair", kind: AreaKindUnknown},
	74:  {name: "Arcane Sanctuary", kind: AreaKindUnknown},
	75:  {name: "Kurast Docktown", kind: AreaKindUnknown},
	76:  {name: "Spider Forest", kind: AreaKindUnknown},
	77:  {name: "Great Marsh", kind: AreaKindUnknown},
	78:  {name: "Flayer Jungle", kind: AreaKindUnknown},
	79:  {name: "Lower Kurast", kind: AreaKindUnknown},
	80:  {name: "Kurast Bazaar", kind: AreaKindUnknown},
	81:  {name: "Upper Kurast", kind: AreaKindUnknown},
	82:  {name: "Kurast Causeway", kind: AreaKindUnknown},
	83:  {name: "Travincal", kind: AreaKindUnknown},
	84:  {name: "Spider Cave", kind: AreaKindUnknown},
	85:  {name: "Spider Cavern", kind: AreaKindUnknown},
	86:  {name: "Swampy Pit Level 1", kind: AreaKindUnknown},
	87:  {name: "Swampy Pit Level 2", kind: AreaKindUnknown},
	88:  {name: "Flayer Dungeon Level 1", kind: AreaKindUnknown},
	89:  {name: "Flayer Dungeon Level 2", kind: AreaKindUnknown},
	90:  {name: "Swampy Pit Level 3", kind: AreaKindUnknown},
	91:  {name: "Flayer Dungeon Level 3", kind: AreaKindUnknown},
	92:  {name: "Sewers Level 1", kind: AreaKindUnknown},
	93:  {name: "Sewers Level 2", kind: AreaKindUnknown},
	94:  {name: "Ruined Temple", kind: AreaKindUnknown},
	95:  {name: "Disused Fane", kind: AreaKindUnknown},
	96:  {name: "Forgotten Reliquary", kind: AreaKindUnknown},
	97:  {name: "Forgotten Temple", kind: AreaKindUnknown},
	98:  {name: "Ruined Fane", kind: AreaKindUnknown},
	99:  {name: "Disused Reliquary", kind: AreaKindUnknown},
	100: {name: "Durance of Hate Level 1", kind: AreaKindDungeon},
	101: {name: "Durance of Hate Level 2", kind: AreaKindDungeon},
	102: {name: "Durance of Hate Level 3", kind: AreaKindDungeon},
	103: {name: "The Pandemonium Fortress", kind: AreaKindUnknown},
	104: {name: "Outer Steppes", kind: AreaKindUnknown},
	105: {name: "Plains of Despair", kind: AreaKindUnknown},
	106: {name: "City of the Damned", kind: AreaKindUnknown},
	107: {name: "River of Flame", kind: AreaKindUnknown},
	108: {name: "Chaos Sanctum", kind: AreaKindUnknown},
	109: {name: "Harrogath", kind: AreaKindUnknown},
	110: {name: "Bloody Foothills", kind: AreaKindUnknown},
	111: {name: "Rigid Highlands", kind: AreaKindUnknown},
	112: {name: "Arreat Plateau", kind: AreaKindUnknown},
	113: {name: "Crystalized Cavern Level 1", kind: AreaKindUnknown},
	114: {name: "Cellar of Pity", kind: AreaKindUnknown},
	115: {name: "Crystalized Cavern Level 2", kind: AreaKindUnknown},
	116: {name: "Echo Chamber", kind: AreaKindUnknown},
	117: {name: "Tundra Wastelands", kind: AreaKindUnknown},
	118: {name: "Glacial Caves Level 1", kind: AreaKindUnknown},
	119: {name: "Glacial Caves Level 2", kind: AreaKindUnknown},
	120: {name: "Rocky Summit", kind: AreaKindUnknown},
	121: {name: "Nihlathaks Temple", kind: AreaKindUnknown},
	122: {name: "Halls of Anguish", kind: AreaKindUnknown},
	123: {name: "Halls of Death's Calling", kind: AreaKindUnknown},
	124: {name: "Halls of Vaught", kind: AreaKindUnknown},
	125: {name: "Hell1", kind: AreaKindUnknown},
	126: {name: "Hell2", kind: AreaKindUnknown},
	127: {name: "Hell3", kind: AreaKindUnknown},
	128: {name: "The Worldstone Keep Level 1", kind: AreaKindUnknown},
	129: {name: "The Worldstone Keep Level 2", kind: AreaKindUnknown},
	130: {name: "The Worldstone Keep Level 3", kind: AreaKindUnknown},
	131: {name: "Throne of Destruction", kind: AreaKindUnknown},
	132: {name: "The Worldstone Chamber", kind: AreaKindUnknown},
	133: {name: "Pandemonium Run 1", kind: AreaKindUnknown},
	134: {name: "Pandemonium Run 2", kind: AreaKindUnknown},
	135: {name: "Pandemonium Run 3", kind: AreaKindUnknown},
	136: {name: "Tristram", kind: AreaKindUnknown},
}
