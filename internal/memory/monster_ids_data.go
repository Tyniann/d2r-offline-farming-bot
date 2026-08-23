// Code generated from D2R 3.2.92777 local data/global/excel/monstats.txt and superuniques.txt; DO NOT EDIT.
package memory

const (
	phase20NPCIDStonySkeleton          uint32 = 0
	phase20NPCIDTristramReturned       uint32 = 1
	phase20NPCIDStonyHungryDead        uint32 = 6
	phase20NPCIDStonyFoulCrow          uint32 = 15
	phase20NPCIDRakanishu              uint32 = 20
	phase20NPCIDStonyMoonClan          uint32 = 53
	phase20NPCIDTristramNightClan      uint32 = 54
	phase20NPCIDTristramCarverShaman   uint32 = 59
	phase20NPCIDStonyDarkRanger        uint32 = 160
	phase20NPCIDTristramSkeletonArcher uint32 = 170
	phase20NPCIDStonyFoulCrowNest      uint32 = 206
	phase20NPCIDHellBovine             uint32 = 391
	phase20NPCIDCowKing                uint32 = 735
)

var runtimeBossNPCIDs = map[uint32]struct{}{
	242: {}, // Mephisto
	250: {}, // Summoner
	526: {}, // Nihlathak
}

var runtimeLowerKurastMonsterNPCIDs = map[uint32]struct{}{
	51:  {}, // DoomApe (baboon4)
	81:  {}, // TreeLurker (sandleaper4)
	112: {}, // HellBuzzard (vulture3)
	235: {}, // Zakarumite (zealot1)
}

var runtimePhase20MonsterNPCIDs = map[uint32]struct{}{
	0:   {}, // StonySkeleton (skeleton1)
	1:   {}, // TristramReturned (skeleton2)
	6:   {}, // StonyHungryDead (zombie2)
	15:  {}, // StonyFoulCrow (foulcrow1)
	20:  {}, // Rakanishu (fallen2)
	53:  {}, // StonyMoonClan (goatman1)
	54:  {}, // TristramNightClan (goatman2)
	59:  {}, // TristramCarverShaman (fallenshaman2)
	160: {}, // StonyDarkRanger (cr_archer1)
	170: {}, // TristramSkeletonArcher (sk_archer1)
	206: {}, // StonyFoulCrowNest (crownest1)
	391: {}, // HellBovine (hellbovine)
	735: {}, // CowKing (cowking)
}

var runtimePhase20PriorityNPCIDs = map[uint32]struct{}{
	20:  {}, // Rakanishu (fallen2)
	735: {}, // CowKing (cowking)
}
