package game

import (
	"math"
	"testing"
)

func TestNewGameLayoutsAndOfficer(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 7},
		Player2: UnitSetup{BaseWidthMM: 50, BaseDepthMM: 50, Count: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Units[0].Minis) != 7 {
		t.Fatalf("got %d minis", len(g.Units[0].Minis))
	}
	officers := 0
	for _, mini := range g.Units[0].Minis {
		if mini.IsOfficer {
			officers++
			if mini.Rank != 0 {
				t.Fatalf("officer not in front rank: %+v", mini)
			}
		}
	}
	if officers != 1 {
		t.Fatalf("got %d officers", officers)
	}
	if g.Units[0].Minis[0].Key != "p1-u1-m01" {
		t.Fatalf("unstable key: %s", g.Units[0].Minis[0].Key)
	}
}

func mathRound(v float64) float64 {
	return math.Round(v*1000000) / 1000000
}

func TestOfficerExistsForSmallUnits(t *testing.T) {
	engine := NewEngine(1)
	for _, count := range []int{1, 2} {
		g, err := engine.NewGame(Setup{
			Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: count},
			Player2: UnitSetup{BaseWidthMM: 50, BaseDepthMM: 100, Count: 1},
		})
		if err != nil {
			t.Fatal(err)
		}
		officers := 0
		for _, mini := range g.Units[0].Minis {
			if mini.IsOfficer {
				officers++
			}
		}
		if officers != 1 {
			t.Fatalf("count %d got %d officers", count, officers)
		}
	}
}

func TestNewGameSetsMovementLimitFromMovementStat(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, Stats: UnitStats{M: 3}},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, Stats: UnitStats{M: 5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.Units[0].MovementLimitMM != 75 {
		t.Fatalf("player 1 movement limit = %d, want M 3 * 25mm", g.Units[0].MovementLimitMM)
	}
	if g.Units[1].MovementLimitMM != 125 {
		t.Fatalf("player 2 movement limit = %d, want M 5 * 25mm", g.Units[1].MovementLimitMM)
	}
}

func TestNewGameCarriesRosterCurrentHealthIntoMinis(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, MaxHealth: 4, CurrentHealth: 2, CurrentHealthSet: true, Stats: UnitStats{H: 4}},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, MaxHealth: 4, CurrentHealth: 4, CurrentHealthSet: true, Stats: UnitStats{H: 4}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, mini := range g.Units[0].Minis {
		if mini.HealthRemaining != 2 {
			t.Fatalf("mini health = %d, want roster current health 2", mini.HealthRemaining)
		}
	}
}

func TestNewGameCarriesZeroRosterHealthAsRemovedMinis(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, MaxHealth: 4, CurrentHealth: 0, CurrentHealthSet: true, Stats: UnitStats{H: 4}},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, MaxHealth: 4, CurrentHealth: 4, CurrentHealthSet: true, Stats: UnitStats{H: 4}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if activeMiniCount(g.Units[0]) != 0 {
		t.Fatalf("zero-health roster unit active minis = %d, want 0", activeMiniCount(g.Units[0]))
	}
	for _, mini := range g.Units[0].Minis {
		if mini.HealthRemaining != 0 || !mini.Removed {
			t.Fatalf("zero-health mini should remain removed, got %+v", mini)
		}
	}
	if unitID, ok := placementUnitID(g); !ok || unitID != "u2" {
		t.Fatalf("placement should skip zero-health unit, got %q ok=%v", unitID, ok)
	}
	if g.ActivePlayer != 2 {
		t.Fatalf("active player = %d, want only fieldable player 2", g.ActivePlayer)
	}
	if _, err := engine.PlaceUnit(g, PlacementRequest{PlayerID: 2, UnitID: "u2", X: 300, Y: 300}); err != nil {
		t.Fatal(err)
	}
	if g.Phase != "complete" || g.WinnerPlayerID != 2 {
		t.Fatalf("single fieldable player should win after setup: phase=%q winner=%d", g.Phase, g.WinnerPlayerID)
	}
}

func TestNewGameUsesDistinctRandomSeeds(t *testing.T) {
	engine := NewEngine(1)
	first, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.RandomSeed == second.RandomSeed {
		t.Fatalf("game random seeds should differ, both were %d", first.RandomSeed)
	}
}

func TestInvalidBaseAndCount(t *testing.T) {
	engine := NewEngine(1)
	_, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 50, BaseDepthMM: 100, Count: 2},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 1},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestActivationAndMove(t *testing.T) {
	engine := NewEngine(3)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	placeDefaultUnits(g)
	unit := firstUnitForPlayer(g, g.ActivePlayer)
	_, _, err = engine.Activate(g, ActivateRequest{PlayerID: g.ActivePlayer, UnitID: unit.ID})
	if err != nil {
		t.Fatal(err)
	}
	beforeX := unit.X
	beforeY := unit.Y
	_, err = engine.ApplyAction(g, ActionRequest{PlayerID: unit.PlayerID, UnitID: unit.ID, Type: ActionMove, Direction: "forward", DistanceMM: 20})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := findUnit(g, unit.ID)
	if updated.X != beforeX {
		t.Fatalf("facing 0 should not change x: before %v after %v", beforeX, updated.X)
	}
	if updated.Y != beforeY-20 {
		t.Fatalf("facing 0 should move north/up: before %v after %v", beforeY, updated.Y)
	}
}

func TestActivationRollsReplayFromRestoredRandomProgress(t *testing.T) {
	engine := NewEngine(17)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	placeDefaultUnits(g)
	unit := firstUnitForPlayer(g, g.ActivePlayer)
	before, err := Snapshot(g)
	if err != nil {
		t.Fatal(err)
	}
	_, firstRoll, err := engine.Activate(g, ActivateRequest{PlayerID: g.ActivePlayer, UnitID: unit.ID})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Restore(before)
	if err != nil {
		t.Fatal(err)
	}
	_, replayedRoll, err := engine.Activate(restored, ActivateRequest{PlayerID: restored.ActivePlayer, UnitID: unit.ID})
	if err != nil {
		t.Fatal(err)
	}
	if firstRoll[0] != replayedRoll[0] || firstRoll[1] != replayedRoll[1] {
		t.Fatalf("roll replay mismatch: got %v, want %v", replayedRoll, firstRoll)
	}
	if restored.RandomRollIndex != g.RandomRollIndex {
		t.Fatalf("random cursor mismatch: got %d, want %d", restored.RandomRollIndex, g.RandomRollIndex)
	}
}

func TestRestoreNormalizesLegacyCombatDefaults(t *testing.T) {
	restored, err := Restore(`{
		"id":"legacy",
		"round":1,
		"phase":"awaiting_activation",
		"units":[{
			"id":"u1",
			"playerId":1,
			"placed":true,
			"stats":{"h":2},
			"base":{"widthMm":25,"depthMm":25,"perRank":5},
			"minis":[{"key":"m1","widthMm":25,"depthMm":25}]
		}]
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Engagements == nil {
		t.Fatal("legacy restore should normalize engagements to an empty slice")
	}
	if restored.Units[0].Minis[0].HealthRemaining != 2 {
		t.Fatalf("legacy mini health got %d, want 2", restored.Units[0].Minis[0].HealthRemaining)
	}
}

func TestCompassFacingMovement(t *testing.T) {
	cases := []struct {
		name   string
		facing int
		wantX  float64
		wantY  float64
	}{
		{name: "north", facing: 0, wantX: 100, wantY: 80},
		{name: "east", facing: 90, wantX: 120, wantY: 100},
		{name: "south", facing: 180, wantX: 100, wantY: 120},
		{name: "west", facing: 270, wantX: 80, wantY: 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			unit := Unit{X: 100, Y: 100, FacingDeg: tc.facing, MovementLimitMM: MovementLimitMM}
			act := &Activation{}
			_, err := applyMove(&unit, act, ActionRequest{Type: ActionMove, Direction: "forward", DistanceMM: 20}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if mathRound(unit.X) != tc.wantX || mathRound(unit.Y) != tc.wantY {
				t.Fatalf("got (%v,%v), want (%v,%v)", unit.X, unit.Y, tc.wantX, tc.wantY)
			}
		})
	}
}

func TestHitsDamageHighestNumberedNonOfficerBeforeOfficer(t *testing.T) {
	unit := formationUnit("u1", 1, 100, 100, 0, 5)
	unit.Stats.H = 2
	for i := range unit.Minis {
		unit.Minis[i].HealthRemaining = 2
	}

	result := applyHitsToUnit(&unit, 9)

	if result.Damage != 9 || result.Removed != 4 {
		t.Fatalf("hit application got damage %d removed %d", result.Damage, result.Removed)
	}
	if len(result.Casualties) == 0 || result.Casualties[0].HealthBefore != 2 || result.Casualties[0].HealthAfter != 1 {
		t.Fatalf("casualty health feedback missing before/after detail: %+v", result.Casualties)
	}
	for _, index := range []int{1, 2, 4, 5} {
		mini := unit.Minis[index-1]
		if !mini.Removed || mini.HealthRemaining != 0 {
			t.Fatalf("mini %d should be removed last-to-first before officer: %+v", index, mini)
		}
	}
	officer := unit.Minis[2]
	if !officer.IsOfficer || officer.Removed || officer.HealthRemaining != 1 {
		t.Fatalf("officer should receive only final leftover damage: %+v", officer)
	}
}

func TestCombatDiceCountUsesActiveFrontRankOrFullRanks(t *testing.T) {
	unit := formationUnit("u1", 1, 100, 100, 0, 10)
	unit.Stats.CD = 2

	if got := combatDiceCount(unit, CombatFaceFront); got != 10 {
		t.Fatalf("front dice got %d, want 10", got)
	}
	if got := combatDiceCount(unit, CombatFaceLeft); got != 4 {
		t.Fatalf("side dice got %d, want 4", got)
	}

	unit.Minis[4].Removed = true
	if got := combatDiceCount(unit, CombatFaceFront); got != 8 {
		t.Fatalf("front dice after casualty got %d, want 8", got)
	}
	if got := combatDiceCount(unit, CombatFaceRight); got != 2 {
		t.Fatalf("side dice after broken rank got %d, want 2", got)
	}
}

func TestCombatTargetNumberRecordsRuleModifiersAndHits(t *testing.T) {
	g := &Game{
		Round: 1,
		ActionHistory: []ActionRecord{
			{Round: 1, Type: ActionActivate, UnitID: "u1"},
		},
	}
	attacker := formationUnit("u1", 1, 100, 100, 0, 10)
	attacker.Stats.A = 6
	attacker.Disordered = true
	defender := formationUnit("u2", 2, 100, 50, 180, 5)
	defender.Stats.D = 11

	target, modifiers := combatTargetNumber(g, attacker, defender, CombatFaceFront, CombatFaceRear, "u2", false)

	if target != 5 {
		t.Fatalf("target got %d, want 5; modifiers=%+v", target, modifiers)
	}
	for _, label := range []string{"ranks", "attacking flank or rear", "defender rear face", "attacker disordered"} {
		if !hasCombatModifier(modifiers, label) {
			t.Fatalf("missing modifier %q in %+v", label, modifiers)
		}
	}
	for _, tc := range []struct {
		roll int
		want int
	}{
		{roll: 4, want: 0},
		{roll: 5, want: 1},
		{roll: 10, want: 2},
		{roll: 15, want: 3},
	} {
		if got := hitsForRoll(tc.roll, 5); got != tc.want {
			t.Fatalf("roll %d got %d hit(s), want %d", tc.roll, got, tc.want)
		}
	}
}

func TestElfGoblinCombatTargetsUseDefenseMinusAttack(t *testing.T) {
	elf := formationUnit("u1", 1, 100, 100, 270, 5)
	elf.Stats = UnitStats{A: 3, D: 9, CD: 1, H: 1}
	goblin := formationUnit("u2", 2, 100, 200, 90, 10)
	goblin.Stats = UnitStats{A: 6, D: 8, CD: 1, H: 1}

	elfTarget, elfModifiers := combatTargetNumber(&Game{Round: 1}, elf, goblin, CombatFaceFront, CombatFaceFront, "u1", false)
	if elfTarget != 5 {
		t.Fatalf("elf target got %d, want 5; modifiers=%+v", elfTarget, elfModifiers)
	}

	goblinTarget, goblinModifiers := combatTargetNumber(&Game{Round: 1}, goblin, elf, CombatFaceFront, CombatFaceFront, "u1", false)
	if goblinTarget != 2 {
		t.Fatalf("goblin target got %d, want 2; modifiers=%+v", goblinTarget, goblinModifiers)
	}

	elfRolls := []int{4, 1, 3, 5, 8}
	elfHits := 0
	for _, roll := range elfRolls {
		elfHits += hitsForRoll(roll, elfTarget)
	}
	if elfHits != 2 {
		t.Fatalf("elf hits got %d, want 2", elfHits)
	}

	goblinRolls := []int{4, 1, 1, 5, 6}
	goblinHits := 0
	for _, roll := range goblinRolls {
		goblinHits += hitsForRoll(roll, goblinTarget)
	}
	if goblinHits != 3 {
		t.Fatalf("goblin hits got %d, want 3", goblinHits)
	}
}

func TestCombatMessagesIncludeTargetNumbers(t *testing.T) {
	messages := combatMessages(CombatRoundResult{
		EngagementID: "combat-test",
		Attacker: CombatSideResult{
			UnitID:       "elf",
			Rolls:        []int{4, 1, 3, 5, 8},
			TargetNumber: 5,
			Hits:         2,
		},
		Defender: CombatSideResult{
			UnitID:       "goblin",
			Rolls:        []int{4, 1, 1, 5, 6},
			TargetNumber: 2,
			Hits:         3,
		},
	})

	if len(messages) == 0 || messages[0] != "Combat combat-test: elf rolled [4 1 3 5 8] vs TN 5 for 2 hit(s); goblin rolled [4 1 1 5 6] vs TN 2 for 3 hit(s)." {
		t.Fatalf("combat message missing target numbers: %#v", messages)
	}
}

func TestMoveIntoCombatCreatesFlushEngagementAndCombatResult(t *testing.T) {
	engine := NewEngine(23)
	g := &Game{
		Round:         1,
		Phase:         "activated",
		ActivePlayer:  1,
		RandomSeed:    23,
		Battlemap:     Battlemaps()[0],
		Engagements:   []CombatEngagement{},
		ActionHistory: []ActionRecord{},
	}
	attacker := formationUnit("u1", 1, 100, 100, 0, 1)
	attacker.Stats = UnitStats{A: 20, D: 20, CD: 1, H: 20}
	setMiniHealth(&attacker, 20)
	defender := formationUnit("u2", 2, 100, 50, 0, 1)
	defender.Stats = UnitStats{A: 1, D: 20, CD: 1, H: 20}
	setMiniHealth(&defender, 20)
	g.Units = []Unit{attacker, defender}
	g.CurrentActivation = &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 1}

	rec, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionMove, Direction: "forward", DistanceMM: 40})
	if err != nil {
		t.Fatal(err)
	}

	result := rec.Result.(map[string]any)
	move := result["movement"].(MoveResult)
	if len(g.Engagements) != 1 || !g.Engagements[0].Active {
		t.Fatalf("expected active engagement, got %+v", g.Engagements)
	}
	if g.Engagements[0].DefenderFace != CombatFaceRear {
		t.Fatalf("defender face got %q, want rear", g.Engagements[0].DefenderFace)
	}
	updated, _ := findUnit(g, "u1")
	if updated.FacingDeg != 0 || unitsMiniRectsOverlap(*updated, updated.X, updated.Y, defender) {
		t.Fatalf("attacker should be flush and non-overlapping: %+v", updated)
	}
	if move.Status != "entered_combat" || move.Combat == nil {
		t.Fatalf("move result did not record combat: %+v", move)
	}
	if len(move.Combat.Attacker.Rolls) != 1 || len(move.Combat.Defender.Rolls) != 1 {
		t.Fatalf("combat rolls not recorded: %+v", move.Combat)
	}
}

func TestAngledSideContactEntersCombat(t *testing.T) {
	engine := NewEngine(44)
	g := &Game{
		Round:         3,
		Phase:         "activated",
		ActivePlayer:  1,
		RandomSeed:    44,
		Battlemap:     Battlemaps()[0],
		Engagements:   []CombatEngagement{},
		ActionHistory: []ActionRecord{},
	}
	attacker := formationUnit("u1", 1, 387.192341, 202.457735, 93, 10)
	attacker.MovementLimitMM = 150
	attacker.Stats = UnitStats{A: 20, D: 20, CD: 1, H: 20}
	setMiniHealth(&attacker, 20)
	defender := formationUnit("u2", 2, 432.4838532554112, 429.9246441530356, 267, 10)
	defender.MovementLimitMM = 125
	defender.Stats = UnitStats{A: 20, D: 20, CD: 1, H: 20}
	setMiniHealth(&defender, 20)
	g.Units = []Unit{attacker, defender}
	g.CurrentActivation = &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 1}

	rec, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionMove, Direction: "forward", DistanceMM: 147})
	if err != nil {
		t.Fatal(err)
	}

	result := rec.Result.(map[string]any)
	move := result["movement"].(MoveResult)
	if move.Status != "entered_combat" {
		t.Fatalf("move status = %q, want entered_combat: %+v", move.Status, move)
	}
	if move.DefenderFace != CombatFaceRight {
		t.Fatalf("defender face = %q, want right", move.DefenderFace)
	}
	updated, _ := findUnit(g, "u1")
	if unitsMiniRectsOverlap(*updated, updated.X, updated.Y, defender) {
		t.Fatal("attacker should be snapped into non-overlapping combat contact")
	}
	if !unitsMiniRectsTouch(*updated, updated.X, updated.Y, defender) && !unitsMiniRectsNearTouch(*updated, updated.X, updated.Y, defender, combatContactToleranceMM) {
		t.Fatal("attacker should be snapped into combat contact")
	}
}

func TestExactEdgeContactEntersCombat(t *testing.T) {
	engine := NewEngine(24)
	g := &Game{
		Round:         1,
		Phase:         "activated",
		ActivePlayer:  1,
		RandomSeed:    24,
		Battlemap:     Battlemaps()[0],
		Engagements:   []CombatEngagement{},
		ActionHistory: []ActionRecord{},
	}
	attacker := formationUnit("u1", 1, 100, 100, 0, 1)
	attacker.Stats = UnitStats{F: 1, D: 1, CD: 1, H: 20}
	defender := formationUnit("u2", 2, 100, 50, 0, 1)
	defender.Stats = UnitStats{F: 1, D: 1, CD: 1, H: 20}
	g.Units = []Unit{attacker, defender}
	g.CurrentActivation = &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 1}

	rec, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionMove, Direction: "forward", DistanceMM: 25})
	if err != nil {
		t.Fatal(err)
	}
	move := rec.Result.(map[string]any)["movement"].(MoveResult)
	if move.Status != "entered_combat" || len(g.Engagements) != 1 {
		t.Fatalf("exact edge contact should enter combat: move=%+v engagements=%+v", move, g.Engagements)
	}
}

func TestMoveIntoCombatAcrossPassableObstacleAddsFortificationModifier(t *testing.T) {
	engine := NewEngine(31)
	g := &Game{
		Round:        1,
		Phase:        "activated",
		ActivePlayer: 1,
		RandomSeed:   31,
		Battlemap: Battlemap{Terrains: []TerrainZone{
			{ID: "fence", Type: TerrainPassableObstacle, Label: "fence", Shape: "rect", X: 95, Y: 70, Width: 35, Height: 8},
		}},
		Engagements:   []CombatEngagement{},
		ActionHistory: []ActionRecord{},
	}
	attacker := formationUnit("u1", 1, 100, 100, 0, 1)
	attacker.Stats = UnitStats{A: 20, D: 20, CD: 1, H: 20}
	setMiniHealth(&attacker, 20)
	defender := formationUnit("u2", 2, 100, 50, 0, 1)
	defender.Stats = UnitStats{A: 1, D: 20, CD: 1, H: 20}
	setMiniHealth(&defender, 20)
	g.Units = []Unit{attacker, defender}
	g.CurrentActivation = &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 1}

	rec, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionMove, Direction: "forward", DistanceMM: 40})
	if err != nil {
		t.Fatal(err)
	}

	if len(g.Engagements) != 1 || !g.Engagements[0].DefenderFortified {
		t.Fatalf("engagement should record defender fortification: %+v", g.Engagements)
	}
	result := rec.Result.(map[string]any)
	move := result["movement"].(MoveResult)
	if move.Combat == nil || !hasCombatModifier(move.Combat.Attacker.Modifiers, "defender behind fortification") {
		t.Fatalf("attacker combat result missing fortification modifier: %+v", move.Combat)
	}
}

func TestCombatChoicePushbackMovesLoserAndClosesEngagement(t *testing.T) {
	engine := NewEngine(37)
	g := combatChoiceGame()
	loserBeforeY := g.Units[1].Y

	rec, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionCombatPushback, CombatChoice: CombatChoicePushback25})
	if err != nil {
		t.Fatal(err)
	}

	if rec.Type != ActionCombatPushback || g.PendingCombatChoice != nil {
		t.Fatalf("choice should record and clear pending state: rec=%+v pending=%+v", rec, g.PendingCombatChoice)
	}
	if g.Engagements[0].Active {
		t.Fatalf("resolved choice should close engagement: %+v", g.Engagements[0])
	}
	if got := mathRound(g.Units[1].Y - loserBeforeY); got != -25 {
		t.Fatalf("loser moved %.0fmm, want -25mm", got)
	}
	choiceResult := rec.Result.(map[string]any)["combatChoice"].(CombatChoiceResult)
	if choiceResult.MovingUnitID != "u2" || choiceResult.RequestedDistanceMM != 25 || choiceResult.MovedDistanceMM != 25 || choiceResult.StoppedBy != "completed" {
		t.Fatalf("structured choice result missing movement detail: %+v", choiceResult)
	}
}

func TestCombatChoiceMovementStopsAtBattlemapEdge(t *testing.T) {
	engine := NewEngine(37)
	g := combatChoiceGame()
	g.Battlemap = Battlemap{ID: "small", Name: "Small", WidthMM: 760, HeightMM: 520}
	g.Units[1].Y = 10

	rec, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionCombatPushback, CombatChoice: CombatChoicePushback25})
	if err != nil {
		t.Fatal(err)
	}
	choiceResult := rec.Result.(map[string]any)["combatChoice"].(CombatChoiceResult)
	if choiceResult.MovedDistanceMM != 10 || choiceResult.StoppedBy != "obstacle_or_arena" {
		t.Fatalf("choice movement = %+v, want 10mm stopped by arena edge", choiceResult)
	}
}

func TestCombatChoiceWithdrawMovesWinnerAndRejectsWrongUnit(t *testing.T) {
	engine := NewEngine(38)
	g := combatChoiceGame()

	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u2", Type: ActionCombatPushback, CombatChoice: CombatChoiceDecline}); err == nil {
		t.Fatal("expected wrong unit error")
	}

	winnerBeforeY := g.Units[0].Y
	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionCombatPushback, CombatChoice: CombatChoiceWithdraw25}); err != nil {
		t.Fatal(err)
	}
	if got := mathRound(g.Units[0].Y - winnerBeforeY); got != 25 {
		t.Fatalf("winner withdrew %.0fmm, want 25mm", got)
	}
	if g.Engagements[0].Active || g.PendingCombatChoice != nil {
		t.Fatalf("withdraw should close combat state: engagement=%+v pending=%+v", g.Engagements[0], g.PendingCombatChoice)
	}
}

func TestDefenderWonPushbackMovesLosingAttackerAwayFromDefender(t *testing.T) {
	engine := NewEngine(39)
	g := combatChoiceGame()
	g.PendingCombatChoice.WinningPlayerID = 2
	g.PendingCombatChoice.WinningUnitID = "u2"
	g.PendingCombatChoice.WinningIsAttacker = false
	g.PendingCombatChoice.LosingUnitID = "u1"
	attackerBeforeY := g.Units[0].Y

	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: 2, UnitID: "u2", Type: ActionCombatPushback, CombatChoice: CombatChoicePushback25}); err != nil {
		t.Fatal(err)
	}
	if got := mathRound(g.Units[0].Y - attackerBeforeY); got != 25 {
		t.Fatalf("losing attacker moved %.0fmm, want 25mm away from defender", got)
	}
}

func TestActivatingUnitInExistingEngagementResolvesCombat(t *testing.T) {
	engine := NewEngine(41)
	g := combatChoiceGame()
	g.PendingCombatChoice = nil
	g.CurrentActivation = nil
	g.Phase = "awaiting_activation"
	g.ActivePlayer = 2
	g.Engagements[0].Active = true
	g.Units[0].Stats = UnitStats{A: 1, D: 20, CD: 1, H: 20}
	g.Units[1].Stats = UnitStats{A: 20, D: 20, CD: 1, H: 20}

	rec, _, err := engine.Activate(g, ActivateRequest{PlayerID: 2, UnitID: "u2"})
	if err != nil {
		t.Fatal(err)
	}
	result := rec.Result.(map[string]any)
	rounds, ok := result["combatRounds"].([]CombatRoundResult)
	if !ok || len(rounds) != 1 {
		t.Fatalf("activation should include combatRounds: %#v", result)
	}
	if g.PendingCombatChoice == nil || g.Phase != "pending_combat_choice" {
		t.Fatalf("activation combat should create pending choice: phase=%q pending=%+v", g.Phase, g.PendingCombatChoice)
	}
}

func TestBlockedCombatAlignmentRestoresFacing(t *testing.T) {
	attacker := formationUnit("u1", 1, 100, 100, 90, 1)
	defender := formationUnit("u2", 2, 100, 50, 0, 1)
	terrains := []TerrainZone{{
		ID:     "blocked",
		Type:   TerrainImpassable,
		Shape:  "rect",
		X:      0,
		Y:      0,
		Width:  760,
		Height: 520,
	}}

	battlemap := Battlemap{Name: "Blocked", WidthMM: 760, HeightMM: 520, Terrains: terrains}
	if snapAttackerFlush(&attacker, defender, CombatFaceFront, battlemap, []Unit{attacker, defender}) {
		t.Fatal("expected combat alignment to be blocked")
	}
	if attacker.FacingDeg != 90 {
		t.Fatalf("blocked alignment facing = %d, want restored 90", attacker.FacingDeg)
	}
}

func TestActiveUnitRemovedByMoveCombatClearsActivation(t *testing.T) {
	engine := NewEngine(43)
	g := &Game{
		Round:        1,
		Phase:        "activated",
		ActivePlayer: 1,
		RandomSeed:   43,
		Battlemap:    Battlemaps()[0],
		Engagements:  []CombatEngagement{},
	}
	attacker := formationUnit("u1", 1, 100, 100, 0, 1)
	attacker.Stats = UnitStats{A: 1, D: 1, CD: 1, H: 1}
	defender := formationUnit("u2", 2, 100, 50, 0, 1)
	defender.Stats = UnitStats{A: 20, D: 20, CD: 1, H: 20}
	g.Units = []Unit{attacker, defender}
	g.CurrentActivation = &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 2}

	rec, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionMove, Direction: "forward", DistanceMM: 25})
	if err != nil {
		t.Fatal(err)
	}
	unit, _ := findUnit(g, "u1")
	if !unit.Broken || unit.Placed || g.CurrentActivation != nil || g.Phase != "complete" || g.WinnerPlayerID != 2 {
		t.Fatalf("removed active unit should clear activation: unit=%+v activation=%+v phase=%q", unit, g.CurrentActivation, g.Phase)
	}
	result := rec.Result.(map[string]any)
	combat := result["combatRound"].(*CombatRoundResult)
	for _, morale := range combat.MoraleTests {
		if morale.UnitID == "u1" {
			t.Fatalf("destroyed unit should not take morale: %+v", combat.MoraleTests)
		}
	}
	winnerMessage := false
	for _, message := range rec.Messages {
		if message == "Player 2 wins." {
			winnerMessage = true
		}
	}
	if !winnerMessage {
		t.Fatalf("missing winner message: %+v", rec.Messages)
	}
	if _, _, err := engine.Activate(g, ActivateRequest{PlayerID: 1, UnitID: "u1"}); err == nil {
		t.Fatal("expected removed unit activation error")
	}
	if actions := LegalActions(g); len(actions) != 0 {
		t.Fatalf("complete game should have no legal actions, got %v", actions)
	}
}

func TestDestroyedUnitTriggersNearbyFriendlyMoraleCascade(t *testing.T) {
	engine := NewEngine(43)
	attacker := oneMiniUnit("u1", 1, 100, 100, 0)
	attacker.Stats = UnitStats{A: 1, D: 1, CD: 1, H: 1}
	setMiniHealth(&attacker, 1)
	defender := oneMiniUnit("u2", 2, 100, 75, 180)
	defender.Stats = UnitStats{A: 20, D: 20, CD: 1, H: 20}
	setMiniHealth(&defender, 20)
	nearFriend := oneMiniUnit("u3", 1, 297.5, 100, 0)
	nearFriend.Stats = UnitStats{A: 11, D: 1, CD: 1, H: 1}
	nearFriend.Disordered = true
	setMiniHealth(&nearFriend, 1)
	g := &Game{
		Round:      1,
		RandomSeed: 43,
		Battlemap:  Battlemaps()[0],
		Units:      []Unit{attacker, defender, nearFriend},
		Engagements: []CombatEngagement{{
			ID:             "combat-1",
			AttackerUnitID: "u1",
			DefenderUnitID: "u2",
			DefenderFace:   CombatFaceFront,
			Active:         true,
		}},
	}

	result := engine.resolveCombatRound(g, g.Engagements[0], 0, "u1", map[string]bool{})

	destroyed, _ := findUnit(g, "u1")
	cascaded, _ := findUnit(g, "u3")
	if !destroyed.Broken || destroyed.Placed {
		t.Fatalf("destroyed unit state = %+v, want broken and removed from placement", destroyed)
	}
	if !cascaded.Broken || cascaded.Placed {
		t.Fatalf("nearby friendly state = %+v, want broken by cascade", cascaded)
	}
	for _, morale := range result.MoraleTests {
		if morale.UnitID == "u1" {
			t.Fatalf("destroyed unit should not take its own morale test: %+v", result.MoraleTests)
		}
	}
	if len(result.MoraleTests) != 1 || result.MoraleTests[0].UnitID != "u3" || !result.MoraleTests[0].Cascade {
		t.Fatalf("morale tests = %+v, want one cascade for nearby friendly", result.MoraleTests)
	}
}

func TestCompleteGameIfNoActivePlayersRemainEndsInDraw(t *testing.T) {
	g := &Game{
		Round:             1,
		Phase:             "awaiting_activation",
		CurrentActivation: &Activation{UnitID: "u1", PlayerID: 1, ActionsRemaining: 1},
		PendingCombatChoice: &PendingCombatChoice{
			EngagementID:    "combat-1",
			WinningUnitID:   "u1",
			WinningPlayerID: 1,
			LosingUnitID:    "u2",
		},
		Engagements: []CombatEngagement{{ID: "combat-1", Active: true}},
		Units: []Unit{
			oneMiniUnit("u1", 1, 100, 100, 0),
			oneMiniUnit("u2", 2, 100, 125, 180),
		},
	}
	for i := range g.Units {
		g.Units[i].Minis[0].Removed = true
		g.Units[i].Minis[0].HealthRemaining = 0
	}

	message := completeGameIfWon(g)

	if message != "No players have active units remaining; game ends in a draw." {
		t.Fatalf("message = %q", message)
	}
	if g.Phase != "complete" || g.WinnerPlayerID != 0 || g.CurrentActivation != nil || g.PendingCombatChoice != nil {
		t.Fatalf("draw should complete without winner: phase=%q winner=%d activation=%+v pending=%+v", g.Phase, g.WinnerPlayerID, g.CurrentActivation, g.PendingCombatChoice)
	}
	if g.Engagements[0].Active {
		t.Fatalf("draw should deactivate engagements: %+v", g.Engagements)
	}
	if actions := LegalActions(g); len(actions) != 0 {
		t.Fatalf("complete draw should have no legal actions, got %v", actions)
	}
}

func TestMoraleFailureBreaksDisorderedUnitAndCascades(t *testing.T) {
	engine := NewEngine(29)
	g := &Game{RandomSeed: 29, Battlemap: Battlemaps()[0]}
	broken := oneMiniUnit("u1", 1, 100, 100, 0)
	broken.Stats.A = 11
	broken.Disordered = true
	near := oneMiniUnit("u2", 1, 125, 100, 0)
	near.Stats.A = 11
	near.Disordered = true
	nearEnemy := oneMiniUnit("u3", 2, 125, 125, 0)
	nearEnemy.Stats.A = 11
	nearEnemy.Disordered = true
	far := oneMiniUnit("u4", 1, 500, 100, 0)
	far.Stats.A = 11
	far.Disordered = true
	g.Units = []Unit{broken, near, nearEnemy, far}

	morale := engine.resolveMoraleTest(g, &g.Units[0], false)
	if morale.Outcome != UnitStatusBroken || !g.Units[0].Broken || g.Units[0].Placed {
		t.Fatalf("disordered failed morale should break and remove unit: morale=%+v unit=%+v", morale, g.Units[0])
	}
	cascade := engine.resolveBrokenCascade(g, "u1", map[string]bool{})
	if len(cascade) != 1 || cascade[0].UnitID != "u2" || cascade[0].Outcome != UnitStatusBroken {
		t.Fatalf("cascade got %+v, want only nearby u2 broken", cascade)
	}
	if !g.Units[1].Broken || g.Units[2].Broken || g.Units[3].Broken {
		t.Fatalf("cascade affected wrong units: near=%+v enemy=%+v far=%+v", g.Units[1], g.Units[2], g.Units[3])
	}
}

func TestMoraleCascadeSkipsUnitsAlreadyTestedThisRound(t *testing.T) {
	engine := NewEngine(29)
	g := &Game{
		Round:      1,
		RandomSeed: 29,
		Battlemap:  Battlemaps()[0],
		ActionHistory: []ActionRecord{
			{
				Round: 1,
				Result: map[string]any{
					"combatRounds": []CombatRoundResult{
						{MoraleTests: []MoraleTestResult{{UnitID: "u2", Passed: true}}},
					},
				},
			},
		},
	}
	broken := oneMiniUnit("u1", 1, 100, 100, 0)
	broken.Broken = true
	broken.Placed = false
	alreadyTested := oneMiniUnit("u2", 1, 125, 100, 0)
	alreadyTested.Stats.A = 11
	alreadyTested.Disordered = true
	notYetTested := oneMiniUnit("u3", 1, 150, 100, 0)
	notYetTested.Stats.A = 11
	notYetTested.Disordered = true
	g.Units = []Unit{broken, alreadyTested, notYetTested}

	cascade := engine.resolveBrokenCascade(g, "u1", moraleTestedThisRound(g))

	if len(cascade) != 1 || cascade[0].UnitID != "u3" {
		t.Fatalf("cascade got %+v, want only not-yet-tested u3", cascade)
	}
	if g.Units[1].Broken || !g.Units[2].Broken {
		t.Fatalf("wrong cascade state: already=%+v notYet=%+v", g.Units[1], g.Units[2])
	}
}

func TestActivationCombatMoraleTestedOnceAcrossEngagements(t *testing.T) {
	engine := NewEngine(29)
	attacker := oneMiniUnit("u1", 1, 100, 100, 0)
	attacker.Stats = UnitStats{A: 20, D: 1, CD: 1, H: 20}
	attacker.CurrentHealth = 20
	defender := oneMiniUnit("u2", 2, 100, 75, 180)
	defender.Stats = UnitStats{A: 20, D: 1, CD: 1, H: 20}
	defender.CurrentHealth = 20
	g := &Game{
		Round:      1,
		RandomSeed: 29,
		Battlemap:  Battlemaps()[0],
		Units:      []Unit{attacker, defender},
		Engagements: []CombatEngagement{
			{ID: "combat-1", AttackerUnitID: "u1", DefenderUnitID: "u2", DefenderFace: CombatFaceFront, Active: true},
			{ID: "combat-2", AttackerUnitID: "u1", DefenderUnitID: "u2", DefenderFace: CombatFaceFront, Active: true},
		},
	}

	results := engine.resolveCombatsForUnit(g, "u1", 0)

	if len(results) != 2 {
		t.Fatalf("combat rounds = %d, want 2", len(results))
	}
	tested := map[string]int{}
	for _, result := range results {
		for _, morale := range result.MoraleTests {
			tested[morale.UnitID]++
		}
	}
	if tested["u1"] != 1 || tested["u2"] != 1 {
		t.Fatalf("morale tests = %#v, want each engaged unit tested once", tested)
	}
}

func TestNewGameSelectsBattlemapAndKeepsPlacementsOffImpassable(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		BattlemapID: "forest_wall",
		Player1:     UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 12},
		Player2:     UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.Battlemap.ID != "forest_wall" {
		t.Fatalf("got battlemap %q", g.Battlemap.ID)
	}
	if g.Phase != "setup" {
		t.Fatalf("new game should start in setup phase, got %q", g.Phase)
	}
	placeDefaultUnits(g)
	for _, unit := range g.Units {
		if unitOverlapsTerrain(unit, unit.X, unit.Y, g.Battlemap.Terrains, TerrainImpassable) {
			t.Fatalf("%s overlaps impassable terrain at setup", unit.ID)
		}
	}
}

func TestBattlemapValidationRejectsInvalidTerrain(t *testing.T) {
	err := ValidateBattlemap(Battlemap{
		ID:       "bad-map",
		Name:     "Bad Map",
		WidthMM:  100,
		HeightMM: 100,
		Terrains: []TerrainZone{{
			ID:     "bad-zone",
			Type:   TerrainRough,
			Shape:  "rect",
			X:      75,
			Y:      75,
			Width:  50,
			Height: 50,
		}},
	})
	if err == nil {
		t.Fatal("expected out-of-bounds terrain validation error")
	}
}

func TestRestoreNormalizesLegacyBattlemapDimensions(t *testing.T) {
	g, err := Restore(`{"id":"legacy","battlemap":{"id":"old_road","name":"Old Road","terrains":[]}}`)
	if err != nil {
		t.Fatal(err)
	}
	if g.Battlemap.WidthMM != ArenaWidthMM || g.Battlemap.HeightMM != ArenaHeightMM {
		t.Fatalf("legacy battlemap dimensions = %.0fx%.0f, want %.0fx%.0f", g.Battlemap.WidthMM, g.Battlemap.HeightMM, float64(ArenaWidthMM), float64(ArenaHeightMM))
	}
	if len(g.Battlemap.Terrains) != 0 {
		t.Fatalf("legacy terrain slice = %#v, want empty slice", g.Battlemap.Terrains)
	}
}

func TestPlaceUnitUsesCustomBattlemapBoundsAndCenter(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Battlemap: Battlemap{ID: "wide", Name: "Wide", WidthMM: 1200, HeightMM: 520},
		Player1:   UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		Player2:   UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.ActivePlayer = 1
	if _, err := engine.PlaceUnit(g, PlacementRequest{PlayerID: 1, UnitID: "u1", X: 1000, Y: 260}); err != nil {
		t.Fatal(err)
	}
	unit, _ := findUnit(g, "u1")
	if unit.FacingDeg != 270 {
		t.Fatalf("custom center facing = %d, want 270 toward x=600", unit.FacingDeg)
	}
	if _, err := engine.PlaceUnit(g, PlacementRequest{PlayerID: 2, UnitID: "u2", X: 1210, Y: 260}); err == nil {
		t.Fatal("expected placement outside custom battlemap to fail")
	}
}

func TestPlaceUnitCentersOfficerAndAlternatesPlayers(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Player1Units: []UnitSetup{
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		},
		Player2Units: []UnitSetup{
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.ActivePlayer = 1
	g.FirstPlayer = 1
	rec, err := engine.PlaceUnit(g, PlacementRequest{PlayerID: 1, UnitID: "u1", X: 100, Y: 100})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Type != ActionPlace || g.ActivePlayer != 2 {
		t.Fatalf("got action %s active player %d", rec.Type, g.ActivePlayer)
	}
	unit, _ := findUnit(g, "u1")
	officer, _ := pivotAnchor(unit, "")
	officerX, officerY := miniWorldCenter(*unit, officer, unit.FacingDeg)
	if mathRound(officerX) != 100 || mathRound(officerY) != 100 {
		t.Fatalf("officer center got (%v,%v)", officerX, officerY)
	}
	if unit.FacingDeg != 135 {
		t.Fatalf("got facing %d", unit.FacingDeg)
	}
	if _, err := engine.PlaceUnit(g, PlacementRequest{PlayerID: 2, UnitID: "u2", X: 600, Y: 400}); err != nil {
		t.Fatal(err)
	}
	if g.ActivePlayer != 1 {
		t.Fatalf("player 1 should place extra unit, got %d", g.ActivePlayer)
	}
	if _, err := engine.PlaceUnit(g, PlacementRequest{PlayerID: 1, UnitID: "p1-u2", X: 140, Y: 400}); err != nil {
		t.Fatal(err)
	}
	if g.Phase != "awaiting_activation" || g.ActivePlayer != g.FirstPlayer {
		t.Fatalf("setup should complete to first player, phase %q active %d first %d", g.Phase, g.ActivePlayer, g.FirstPlayer)
	}
}

func TestPlaceUnitRejectsImpassableTerrain(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		BattlemapID: "forest_wall",
		Player1:     UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		Player2:     UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.ActivePlayer = 1
	_, err = engine.PlaceUnit(g, PlacementRequest{PlayerID: 1, UnitID: "u1", X: 330, Y: 340})
	if err == nil {
		t.Fatal("expected impassable placement error")
	}
}

func TestPlaceUnitRejectsOtherUnitOverlap(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.ActivePlayer = 1
	if _, err := engine.PlaceUnit(g, PlacementRequest{PlayerID: 1, UnitID: "u1", X: 200, Y: 200}); err != nil {
		t.Fatal(err)
	}
	g.ActivePlayer = 2
	if _, err := engine.PlaceUnit(g, PlacementRequest{PlayerID: 2, UnitID: "u2", X: 200, Y: 200}); err == nil {
		t.Fatal("expected placement overlap error")
	}
}

func TestPlaceUnitAcceptsExplicitFacing(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.ActivePlayer = 1
	facing := 150
	if _, err := engine.PlaceUnit(g, PlacementRequest{PlayerID: 1, UnitID: "u1", X: 130, Y: 130, FacingDeg: &facing}); err != nil {
		t.Fatal(err)
	}
	unit, _ := findUnit(g, "u1")
	if unit.FacingDeg != 150 {
		t.Fatalf("got facing %d", unit.FacingDeg)
	}
}

func TestMovementThroughRoughTerrainHalvesOnlyOverlappingDistance(t *testing.T) {
	unit := Unit{
		ID:              "u1",
		MovementLimitMM: MovementLimitMM,
		X:               0,
		Y:               100,
		FacingDeg:       0,
		Minis: []Mini{{
			Key:     "m1",
			WidthMM: 25,
			DepthMM: 25,
		}},
	}
	moved, err := applyMove(&unit, &Activation{}, ActionRequest{Direction: "forward", DistanceMM: 100}, []TerrainZone{
		{ID: "rough", Type: TerrainRough, Shape: "rect", X: -10, Y: -200, Width: 50, Height: 250},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 75 || unit.Y != 25 {
		t.Fatalf("got moved %.0f y %.0f, want moved 75 y 25", moved, unit.Y)
	}
}

func TestMovementStopsBeforeImpassableOverlap(t *testing.T) {
	unit := Unit{
		ID:              "u1",
		MovementLimitMM: MovementLimitMM,
		X:               0,
		Y:               100,
		FacingDeg:       0,
		Minis: []Mini{{
			Key:     "m1",
			WidthMM: 25,
			DepthMM: 25,
		}},
	}
	moved, err := applyMove(&unit, &Activation{}, ActionRequest{Direction: "forward", DistanceMM: 100}, []TerrainZone{
		{ID: "wall", Type: TerrainImpassable, Shape: "rect", X: -10, Y: -200, Width: 50, Height: 250},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 50 || unit.Y != 50 {
		t.Fatalf("got moved %.0f y %.0f, want moved 50 y 50", moved, unit.Y)
	}
}

func TestMovementUsesCustomBattlemapBounds(t *testing.T) {
	engine := NewEngine(1)
	unit := oneMiniUnit("u1", 1, 1075, 100, 90)
	g := &Game{
		Round:             1,
		Phase:             "activated",
		ActivePlayer:      1,
		Battlemap:         Battlemap{ID: "wide", Name: "Wide", WidthMM: 1200, HeightMM: 520},
		Units:             []Unit{unit},
		CurrentActivation: &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 2},
	}
	rec, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionMove, Direction: "forward", DistanceMM: 100})
	if err != nil {
		t.Fatal(err)
	}
	result := rec.Result.(map[string]any)
	movement := result["movement"].(MoveResult)
	if movement.DistanceMM != 100 {
		t.Fatalf("moved %.0fmm, want full movement beyond old arena width", movement.DistanceMM)
	}
	movedUnit, _ := findUnit(g, "u1")
	if movedUnit.X != 1175 {
		t.Fatalf("unit x = %.0f, want 1175", movedUnit.X)
	}
}

func TestMovementStopsAtCustomBattlemapEdge(t *testing.T) {
	engine := NewEngine(1)
	unit := oneMiniUnit("u1", 1, 1175, 100, 90)
	g := &Game{
		Round:             1,
		Phase:             "activated",
		ActivePlayer:      1,
		Battlemap:         Battlemap{ID: "wide", Name: "Wide", WidthMM: 1200, HeightMM: 520},
		Units:             []Unit{unit},
		CurrentActivation: &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 2},
	}
	rec, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionMove, Direction: "forward", DistanceMM: 100})
	if err != nil {
		t.Fatal(err)
	}
	result := rec.Result.(map[string]any)
	movement := result["movement"].(MoveResult)
	if movement.DistanceMM != 25 {
		t.Fatalf("moved %.0fmm, want stop at custom edge after 25mm", movement.DistanceMM)
	}
	movedUnit, _ := findUnit(g, "u1")
	if movedUnit.X != 1200 {
		t.Fatalf("unit x = %.0f, want 1200", movedUnit.X)
	}
}

func TestMovementMayPassThroughFriendlyUnitWhenItClears(t *testing.T) {
	unit := oneMiniUnit("u1", 1, 0, 100, 0)
	friendly := oneMiniUnit("u2", 1, 0, 50, 0)
	moved, err := applyMove(&unit, &Activation{}, ActionRequest{Direction: "forward", DistanceMM: 100}, nil, []Unit{unit, friendly})
	if err != nil {
		t.Fatal(err)
	}
	if moved != 100 || unit.Y != 0 {
		t.Fatalf("got moved %.0f y %.0f, want moved 100 y 0", moved, unit.Y)
	}
}

func TestMovementStopsBeforeFriendlyUnitWhenItCannotClear(t *testing.T) {
	unit := oneMiniUnit("u1", 1, 0, 100, 0)
	friendly := oneMiniUnit("u2", 1, 0, 50, 0)
	moved, err := applyMove(&unit, &Activation{}, ActionRequest{Direction: "forward", DistanceMM: 60}, nil, []Unit{unit, friendly})
	if err != nil {
		t.Fatal(err)
	}
	if moved != 25 || unit.Y != 75 {
		t.Fatalf("got moved %.0f y %.0f, want moved 25 y 75", moved, unit.Y)
	}
}

func TestMovementStopsBeforeEnemyUnit(t *testing.T) {
	unit := oneMiniUnit("u1", 1, 0, 100, 0)
	enemy := oneMiniUnit("u2", 2, 0, 50, 0)
	moved, err := applyMove(&unit, &Activation{}, ActionRequest{Direction: "forward", DistanceMM: 100}, nil, []Unit{unit, enemy})
	if err != nil {
		t.Fatal(err)
	}
	if moved != 25 || unit.Y != 75 {
		t.Fatalf("got moved %.0f y %.0f, want moved 25 y 75", moved, unit.Y)
	}
}

func TestEastFacingFriendlyUnitStopsBeforeWestmostExtentWhenItCannotClear(t *testing.T) {
	unit := formationUnit("u1", 1, 100, 100, 90, 10)
	friendly := formationUnit("u2", 1, 200, 100, 90, 10)
	moved, err := applyMove(&unit, &Activation{}, ActionRequest{Direction: "forward", DistanceMM: 60}, nil, []Unit{unit, friendly})
	if err != nil {
		t.Fatal(err)
	}
	if moved != 50 || unit.X != 150 {
		t.Fatalf("got moved %.0f x %.0f, want moved 50 x 150", moved, unit.X)
	}
}

func TestEastFacingFriendlyUnitMayPassThroughWhenItClears(t *testing.T) {
	unit := formationUnit("u1", 1, 125, 100, 90, 5)
	friendly := formationUnit("u2", 1, 175, 100, 90, 5)
	moved, err := applyMove(&unit, &Activation{}, ActionRequest{Direction: "forward", DistanceMM: 75}, nil, []Unit{unit, friendly})
	if err != nil {
		t.Fatal(err)
	}
	if moved != 75 || unit.X != 200 {
		t.Fatalf("got moved %.0f x %.0f, want moved 75 x 200", moved, unit.X)
	}
}

func TestEngineMoveNeverLeavesUnitOverlappingFriendlyUnit(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Player1Units: []UnitSetup{
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 10},
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 10},
		},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Units[0] = formationUnit("u1", 1, 100, 250, 90, 10)
	g.Units[1] = formationUnit("p1-u2", 1, 200, 250, 90, 10)
	g.Units[2] = formationUnit("u2", 2, 600, 250, 270, 5)
	g.Phase = "awaiting_activation"
	g.ActivePlayer = 1
	g.CurrentActivation = &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 1}
	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionMove, Direction: "forward", DistanceMM: 100}); err != nil {
		t.Fatal(err)
	}
	moved, _ := findUnit(g, "u1")
	if unitOverlapsAnyUnit(*moved, moved.X, moved.Y, g.Units) {
		t.Fatalf("move left unit overlapping another unit at x=%v y=%v", moved.X, moved.Y)
	}
}

func TestSecondMoveLimit(t *testing.T) {
	engine := NewEngine(9)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	placeDefaultUnits(g)
	unit := firstUnitForPlayer(g, g.ActivePlayer)
	g.CurrentActivation = &Activation{UnitID: unit.ID, PlayerID: unit.PlayerID, Success: true, ActionsRemaining: 2}
	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: unit.PlayerID, UnitID: unit.ID, Type: ActionMove, Direction: "forward", DistanceMM: 80}); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: unit.PlayerID, UnitID: unit.ID, Type: ActionMove, Direction: "forward", DistanceMM: 80}); err == nil {
		t.Fatal("expected second move distance error")
	}
}

func TestSkipUsesAllRemainingActionsAndAdvancesTurn(t *testing.T) {
	engine := NewEngine(9)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	placeDefaultUnits(g)
	g.ActivePlayer = 1
	g.CurrentActivation = &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 2}
	rec, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionSkip})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Type != ActionSkip {
		t.Fatalf("got action %s", rec.Type)
	}
	if g.CurrentActivation != nil {
		t.Fatal("skip should end the current activation")
	}
	if g.ActivePlayer != 2 || g.Phase != "awaiting_activation" {
		t.Fatalf("got active player %d phase %q", g.ActivePlayer, g.Phase)
	}
}

func TestPivotDefaultsToOfficerAsFixedAxis(t *testing.T) {
	engine := NewEngine(5)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 7},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	placeDefaultUnits(g)
	unit := &g.Units[0]
	officer, err := pivotAnchor(unit, "")
	if err != nil {
		t.Fatal(err)
	}
	beforeX, beforeY := miniWorldCenter(*unit, officer, unit.FacingDeg)
	g.CurrentActivation = &Activation{UnitID: unit.ID, PlayerID: unit.PlayerID, Success: true, ActionsRemaining: 1}
	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: unit.PlayerID, UnitID: unit.ID, Type: ActionPivot, FacingDeg: 90}); err != nil {
		t.Fatal(err)
	}
	afterX, afterY := miniWorldCenter(*unit, officer, unit.FacingDeg)
	if mathRound(afterX) != mathRound(beforeX) || mathRound(afterY) != mathRound(beforeY) {
		t.Fatalf("officer moved during pivot: before (%v,%v), after (%v,%v)", beforeX, beforeY, afterX, afterY)
	}
	if unit.FacingDeg != 90 {
		t.Fatalf("got facing %d", unit.FacingDeg)
	}
}

func TestPivotUsesSelectedAnchorAsFixedAxis(t *testing.T) {
	engine := NewEngine(6)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 7},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	placeDefaultUnits(g)
	unit := &g.Units[0]
	anchor := unit.Minis[0]
	beforeX, beforeY := miniWorldCenter(*unit, anchor, unit.FacingDeg)
	g.CurrentActivation = &Activation{UnitID: unit.ID, PlayerID: unit.PlayerID, Success: true, ActionsRemaining: 1}
	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: unit.PlayerID, UnitID: unit.ID, Type: ActionPivot, FacingDeg: 90, AnchorKey: anchor.Key}); err != nil {
		t.Fatal(err)
	}
	afterX, afterY := miniWorldCenter(*unit, anchor, unit.FacingDeg)
	if mathRound(afterX) != mathRound(beforeX) || mathRound(afterY) != mathRound(beforeY) {
		t.Fatalf("selected anchor moved during pivot: before (%v,%v), after (%v,%v)", beforeX, beforeY, afterX, afterY)
	}
}

func TestPivotStopsBeforeOverlappingAdjacentFriendlyUnit(t *testing.T) {
	engine := NewEngine(7)
	g, err := engine.NewGame(Setup{
		Player1Units: []UnitSetup{
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Units[0] = formationUnit("u1", 1, 100, 100, 0, 5)
	g.Units[1] = formationUnit("p1-u2", 1, 100, 125, 0, 5)
	g.Units[2] = formationUnit("u2", 2, 500, 100, 0, 5)
	g.Phase = "awaiting_activation"
	g.CurrentActivation = &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 1}

	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionPivot, FacingDeg: 90}); err != nil {
		t.Fatal(err)
	}
	unit, _ := findUnit(g, "u1")
	if unit.FacingDeg == 90 {
		t.Fatal("pivot reached target despite adjacent friendly unit")
	}
	if unitOverlapsAnyUnit(*unit, unit.X, unit.Y, g.Units) {
		t.Fatalf("pivot left unit overlapping another unit at facing %d", unit.FacingDeg)
	}
}

func TestLegalActionsDoesNotExposeWheel(t *testing.T) {
	g := &Game{CurrentActivation: &Activation{UnitID: "u1"}}
	foundSkip := false
	for _, action := range LegalActions(g) {
		if action == "wheel" {
			t.Fatal("wheel should not be a separate legal action")
		}
		if action == ActionSkip {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Fatal("skip should be legal during activation")
	}
}

func TestLegalActionsExposeShootOnlyWithLegalTarget(t *testing.T) {
	g := shootingTestGame(t, UnitSetup{
		BaseWidthMM: 25, BaseDepthMM: 25, Count: 5,
		Stats:     UnitStats{A: 6, D: 8, CD: 1, H: 1},
		Equipment: []string{"Bow"},
	}, UnitSetup{
		BaseWidthMM: 25, BaseDepthMM: 25, Count: 5,
		Stats: UnitStats{A: 5, D: 8, CD: 1, H: 1},
	})
	if !containsString(LegalActions(g), ActionShoot) {
		t.Fatalf("legal actions = %v, want shoot", LegalActions(g))
	}
	details := LegalActionDetails(g)
	var shoot LegalAction
	for _, detail := range details {
		if detail.Type == ActionShoot {
			shoot = detail
		}
	}
	if len(shoot.Targets) != 1 || shoot.Targets[0].UnitID != "u2" || shoot.Targets[0].Weapon != "Bow" {
		t.Fatalf("shoot details = %+v, want one bow target", shoot)
	}

	g.Units[1].Y = 900
	if containsString(LegalActions(g), ActionShoot) {
		t.Fatalf("shoot should not be legal out of range, actions=%v", LegalActions(g))
	}
}

func TestShootingWeaponAndSpecialAbilityParsing(t *testing.T) {
	unit := Unit{Name: "Elf Ballista", Equipment: []string{"Hand Weapon"}, Special: []string{"Indirect Fire", "Shielding (1)", "Large"}}
	weapon, rangeMM, ok := shootingWeapon(unit)
	if !ok || weapon != "Ballista" || rangeMM != 750 {
		t.Fatalf("weapon=%q range=%.0f ok=%v, want Ballista 750 true", weapon, rangeMM, ok)
	}
	for _, ability := range []string{"Indirect Fire", "Shielding", "Large"} {
		if !hasSpecialAbility(unit, ability) {
			t.Fatalf("expected ability %q in %+v", ability, unit.Special)
		}
	}
}

func TestShootingLineOfSightFrontArcAndBlocking(t *testing.T) {
	g := shootingTestGame(t, UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 1, Stats: UnitStats{A: 6, D: 8, CD: 1, H: 1}, Equipment: []string{"Bow"}}, UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 1, Stats: UnitStats{A: 5, D: 8, CD: 1, H: 1}})
	if !containsString(LegalActions(g), ActionShoot) {
		t.Fatalf("front target should be legal, actions=%v", LegalActions(g))
	}

	g.Units[1].Y = 420
	if containsString(LegalActions(g), ActionShoot) {
		t.Fatalf("target behind front arc should not be legal, actions=%v", LegalActions(g))
	}

	g = shootingTestGameWithBlocker(t, nil)
	if containsString(LegalActions(g), ActionShoot) {
		t.Fatalf("ordinary blocker should block LOS, actions=%v", LegalActions(g))
	}
}

func TestIndirectFireBypassesLineOfSightBlocker(t *testing.T) {
	g := shootingTestGameWithBlocker(t, []string{"Indirect Fire"})
	if !containsString(LegalActions(g), ActionShoot) {
		t.Fatalf("indirect fire should bypass LOS blocker, actions=%v", LegalActions(g))
	}
}

func TestLargeTargetIgnoresOrdinaryLineOfSightBlocker(t *testing.T) {
	g := shootingTestGameWithBlocker(t, nil)
	g.Units[2].Special = []string{"Large"}
	if !containsString(LegalActions(g), ActionShoot) {
		t.Fatalf("large target should ignore ordinary blocker, actions=%v", LegalActions(g))
	}
}

func TestShootActionRejectsSecondShotInActivation(t *testing.T) {
	engine := NewEngine(7)
	g := shootingTestGame(t, UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 1, Stats: UnitStats{A: 20, D: 8, CD: 1, H: 1}, Equipment: []string{"Bow"}}, UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, Stats: UnitStats{A: 11, D: 1, CD: 1, H: 1}})
	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionShoot, TargetUnitID: "u2"}); err != nil {
		t.Fatal(err)
	}
	g.CurrentActivation = &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 1, ShotsTaken: 1}
	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionShoot, TargetUnitID: "u2"}); err == nil {
		t.Fatal("expected second shot in same activation to be rejected")
	}
}

func TestShootingShieldingDiceReductionCasualtiesMoraleAndSnapshot(t *testing.T) {
	engine := NewEngine(3)
	g := shootingTestGame(t, UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, Stats: UnitStats{A: 20, D: 8, CD: 1, H: 1}, Equipment: []string{"Bow"}}, UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, MaxHealth: 3, CurrentHealth: 3, CurrentHealthSet: true, Stats: UnitStats{A: 11, D: 1, CD: 1, H: 3}, Special: []string{"Shielding (1)"}})
	before, err := Snapshot(g)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := engine.ApplyAction(g, ActionRequest{PlayerID: 1, UnitID: "u1", Type: ActionShoot, TargetUnitID: "u2"})
	if err != nil {
		t.Fatal(err)
	}
	result := rec.Result.(map[string]any)["shooting"].(ShootResult)
	if result.DiceCount != 4 {
		t.Fatalf("dice = %d, want 4 after shielding reduction", result.DiceCount)
	}
	if len(result.Casualties) == 0 || len(result.MoraleTests) == 0 {
		t.Fatalf("shooting result should include casualties and morale: %+v", result)
	}
	restored, err := Restore(before)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.ActionHistory) != 0 || restored.CurrentActivation == nil || restored.CurrentActivation.ShotsTaken != 0 {
		t.Fatalf("restored snapshot = actions %d activation %+v, want pre-shot state", len(restored.ActionHistory), restored.CurrentActivation)
	}
}

func TestNewGameSupportsMultipleUnitsPerPlayer(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Player1Units: []UnitSetup{
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
			{BaseWidthMM: 50, BaseDepthMM: 50, Count: 1},
		},
		Player2Units: []UnitSetup{
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
			{BaseWidthMM: 25, BaseDepthMM: 50, Count: 3},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Units) != 4 {
		t.Fatalf("got %d units", len(g.Units))
	}
	if g.Units[0].ID != "u1" || g.Units[1].ID != "p1-u2" || g.Units[2].ID != "u2" || g.Units[3].ID != "p2-u2" {
		t.Fatalf("unexpected unit ids: %+v", []string{g.Units[0].ID, g.Units[1].ID, g.Units[2].ID, g.Units[3].ID})
	}
}

func TestUnevenUnitCountsFinishRemainingActivationsBeforeNewRound(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Player1Units: []UnitSetup{
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		},
		Player2Units: []UnitSetup{
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	placeDefaultUnits(g)
	g.ActivePlayer = 1

	finishActivation(t, engine, g, "u1", 1)
	if g.ActivePlayer != 2 || g.Round != 1 {
		t.Fatalf("after p1 first activation got player %d round %d", g.ActivePlayer, g.Round)
	}
	finishActivation(t, engine, g, "u2", 2)
	if g.ActivePlayer != 1 || g.Round != 1 {
		t.Fatalf("player 1 should finish extra units, got player %d round %d", g.ActivePlayer, g.Round)
	}
	finishActivation(t, engine, g, "p1-u2", 1)
	if g.Round != 2 || g.ActivePlayer != g.FirstPlayer {
		t.Fatalf("round should reset after all units activate, got player %d round %d first %d", g.ActivePlayer, g.Round, g.FirstPlayer)
	}
}

func TestCannotActivateAlreadyActivatedUnitWhenMultipleChoicesExist(t *testing.T) {
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Player1Units: []UnitSetup{
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		},
		Player2Units: []UnitSetup{
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	placeDefaultUnits(g)
	g.ActivePlayer = 1
	finishActivation(t, engine, g, "u1", 1)
	g.ActivePlayer = 1
	if _, _, err := engine.Activate(g, ActivateRequest{PlayerID: 1, UnitID: "u1"}); err == nil {
		t.Fatal("expected already activated error")
	}
	if _, _, err := engine.Activate(g, ActivateRequest{PlayerID: 1, UnitID: "p1-u2"}); err != nil {
		t.Fatal(err)
	}
}

func TestAboutFaceSwapsOfficerWithLastFullRankAndKeepsPartialRankBack(t *testing.T) {
	engine := NewEngine(4)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 12},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	placeDefaultUnits(g)
	unit := &g.Units[0]
	officer, ok := miniByKey(unit, "p1-u1-m03")
	if !ok {
		t.Fatal("missing officer mini")
	}
	swapTarget, ok := miniByKey(unit, "p1-u1-m08")
	if !ok {
		t.Fatal("missing last-full-rank target")
	}
	if !officer.IsOfficer || swapTarget.IsOfficer {
		t.Fatal("unexpected initial officer assignment")
	}
	expectedOfficerX, expectedOfficerY := miniWorldCenter(*unit, swapTarget, unit.FacingDeg)
	g.CurrentActivation = &Activation{UnitID: unit.ID, PlayerID: unit.PlayerID, Success: true, ActionsRemaining: 1}
	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: unit.PlayerID, UnitID: unit.ID, Type: ActionAboutFace}); err != nil {
		t.Fatal(err)
	}
	updated, _ := findUnit(g, unit.ID)
	officers := 0
	positions := map[[2]float64]bool{}
	for _, mini := range updated.Minis {
		if mini.IsOfficer {
			officers++
			if mini.Rank != 0 {
				t.Fatalf("officer moved out of front rank: %+v", mini)
			}
		}
		key := [2]float64{mini.RelX, mini.RelY}
		if positions[key] {
			t.Fatalf("overlapping mini at %+v", key)
		}
		positions[key] = true
	}
	if officers != 1 {
		t.Fatalf("got %d officers", officers)
	}
	officer, _ = miniByKey(updated, "p1-u1-m03")
	if !officer.IsOfficer || officer.Rank != 0 || officer.File != 2 {
		t.Fatalf("officer not in new front rank same file: %+v", officer)
	}
	officerX, officerY := miniWorldCenter(*updated, officer, updated.FacingDeg)
	if mathRound(officerX) != mathRound(expectedOfficerX) || mathRound(officerY) != mathRound(expectedOfficerY) {
		t.Fatalf("officer did not pivot from swapped position: got (%v,%v), want (%v,%v)", officerX, officerY, expectedOfficerX, expectedOfficerY)
	}
	oldOfficerPlaceMini, _ := miniByKey(updated, "p1-u1-m08")
	if oldOfficerPlaceMini.Rank != 1 || oldOfficerPlaceMini.File != 2 {
		t.Fatalf("swap target did not move into old officer row: %+v", oldOfficerPlaceMini)
	}
	partialA, _ := miniByKey(updated, "p1-u1-m11")
	partialB, _ := miniByKey(updated, "p1-u1-m12")
	if partialA.Rank != 2 || partialA.File != 0 || partialB.Rank != 2 || partialB.File != 1 {
		t.Fatalf("partial rank not left-flushed at back: %+v %+v", partialA, partialB)
	}
	if updated.FacingDeg != 180 {
		t.Fatalf("got facing %d", updated.FacingDeg)
	}
}

func TestAboutFaceRejectsInvalidResultAndKeepsUnitUnchanged(t *testing.T) {
	engine := NewEngine(4)
	g, err := engine.NewGame(Setup{
		Player1: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 12},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	placeDefaultUnits(g)
	unit := &g.Units[0]
	blockedZone := TerrainZone{ID: "blocked-about-face", Type: TerrainImpassable, Shape: "rect", X: 120, Y: 105, Width: 125, Height: 25}
	g.Battlemap.Terrains = []TerrainZone{blockedZone}
	candidate := cloneUnit(*unit)
	if err := applyAboutFace(&candidate); err != nil {
		t.Fatal(err)
	}
	if unitOverlapsTerrain(*unit, unit.X, unit.Y, g.Battlemap.Terrains, TerrainImpassable) {
		t.Fatal("test setup overlaps before about face")
	}
	if !unitOverlapsTerrain(candidate, candidate.X, candidate.Y, g.Battlemap.Terrains, TerrainImpassable) {
		t.Fatal("test setup does not block about face result")
	}
	before := cloneUnit(*unit)
	g.CurrentActivation = &Activation{UnitID: unit.ID, PlayerID: unit.PlayerID, Success: true, ActionsRemaining: 1}

	if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: unit.PlayerID, UnitID: unit.ID, Type: ActionAboutFace}); err == nil {
		t.Fatal("expected about face validation error")
	}
	updated, _ := findUnit(g, unit.ID)
	if updated.X != before.X || updated.Y != before.Y || updated.FacingDeg != before.FacingDeg {
		t.Fatalf("unit moved despite rejected about face: got (%v,%v,%d), want (%v,%v,%d)", updated.X, updated.Y, updated.FacingDeg, before.X, before.Y, before.FacingDeg)
	}
	for i := range updated.Minis {
		if updated.Minis[i].Rank != before.Minis[i].Rank || updated.Minis[i].File != before.Minis[i].File {
			t.Fatalf("mini %d changed despite rejected about face: got %+v want %+v", i, updated.Minis[i], before.Minis[i])
		}
	}
}

func firstUnitForPlayer(g *Game, playerID int) Unit {
	for _, unit := range g.Units {
		if unit.PlayerID == playerID {
			return unit
		}
	}
	return Unit{}
}

func placeDefaultUnits(g *Game) {
	p1 := 0
	p2 := 0
	for i := range g.Units {
		unit := &g.Units[i]
		switch unit.PlayerID {
		case 1:
			unit.X = 120
			unit.Y = float64(130 + p1*115)
			unit.FacingDeg = 0
			p1++
		case 2:
			unit.X = 520
			unit.Y = float64(360 - p2*115)
			unit.FacingDeg = 180
			p2++
		}
		unit.Placed = true
	}
	g.Phase = "awaiting_activation"
	g.ActivePlayer = g.FirstPlayer
}

func oneMiniUnit(id string, playerID int, x, y float64, facing int) Unit {
	return Unit{
		ID:              id,
		PlayerID:        playerID,
		MovementLimitMM: MovementLimitMM,
		X:               x,
		Y:               y,
		FacingDeg:       facing,
		Placed:          true,
		Minis: []Mini{{
			Key:     id + "-m1",
			WidthMM: 25,
			DepthMM: 25,
		}},
	}
}

func formationUnit(id string, playerID int, x, y float64, facing, count int) Unit {
	base, _ := Base(25, 25)
	unit := Unit{
		ID:              id,
		PlayerID:        playerID,
		Base:            base,
		MovementLimitMM: MovementLimitMM,
		X:               x,
		Y:               y,
		FacingDeg:       facing,
		Placed:          true,
	}
	unit.Minis = layoutMinis(unit, count)
	return unit
}

func combatChoiceGame() *Game {
	attacker := formationUnit("u1", 1, 100, 75, 0, 1)
	attacker.Stats = UnitStats{A: 5, F: 1, D: 1, CD: 1, H: 20}
	setMiniHealth(&attacker, 20)
	defender := formationUnit("u2", 2, 100, 50, 0, 1)
	defender.Stats = UnitStats{A: 5, F: 1, D: 1, CD: 1, H: 20}
	setMiniHealth(&defender, 20)
	engagement := CombatEngagement{
		ID:                 "combat-test",
		AttackerUnitID:     "u1",
		DefenderUnitID:     "u2",
		DefenderFace:       CombatFaceRear,
		AxisDX:             0,
		AxisDY:             -1,
		Round:              1,
		CreatedActionIndex: 0,
		Active:             true,
	}
	return &Game{
		Round:               1,
		Phase:               "pending_combat_choice",
		ActivePlayer:        1,
		RandomSeed:          37,
		Battlemap:           Battlemaps()[0],
		Units:               []Unit{attacker, defender},
		Engagements:         []CombatEngagement{engagement},
		PendingCombatChoice: createPendingCombatChoice(engagement, attacker, defender, 0),
		CurrentActivation:   &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 0},
	}
}

func setMiniHealth(unit *Unit, health int) {
	for i := range unit.Minis {
		unit.Minis[i].HealthRemaining = health
	}
}

func finishActivation(t *testing.T, engine *Engine, g *Game, unitID string, playerID int) {
	t.Helper()
	if _, _, err := engine.Activate(g, ActivateRequest{PlayerID: playerID, UnitID: unitID}); err != nil {
		t.Fatal(err)
	}
	unit, ok := findUnit(g, unitID)
	if !ok {
		t.Fatalf("missing unit %s", unitID)
	}
	for g.CurrentActivation != nil {
		if _, err := engine.ApplyAction(g, ActionRequest{PlayerID: playerID, UnitID: unit.ID, Type: ActionPivot, FacingDeg: unit.FacingDeg}); err != nil {
			t.Fatal(err)
		}
	}
}

func shootingTestGame(t *testing.T, attackerSetup, targetSetup UnitSetup) *Game {
	t.Helper()
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{Player1: attackerSetup, Player2: targetSetup})
	if err != nil {
		t.Fatal(err)
	}
	g.Phase = "activated"
	g.ActivePlayer = 1
	g.CurrentActivation = &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 2}
	g.Units[0].Placed = true
	g.Units[0].X = 100
	g.Units[0].Y = 300
	g.Units[0].FacingDeg = 0
	g.Units[1].Placed = true
	g.Units[1].X = 100
	g.Units[1].Y = 100
	g.Units[1].FacingDeg = 180
	return g
}

func shootingTestGameWithBlocker(t *testing.T, attackerSpecial []string) *Game {
	t.Helper()
	engine := NewEngine(1)
	g, err := engine.NewGame(Setup{
		Player1Units: []UnitSetup{
			{BaseWidthMM: 25, BaseDepthMM: 25, Count: 1, Stats: UnitStats{A: 6, D: 8, CD: 1, H: 1}, Equipment: []string{"Bow"}, Special: attackerSpecial},
			{BaseWidthMM: 100, BaseDepthMM: 50, Count: 1, Stats: UnitStats{A: 5, D: 8, CD: 1, H: 1}},
		},
		Player2: UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 1, Stats: UnitStats{A: 5, D: 8, CD: 1, H: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Phase = "activated"
	g.ActivePlayer = 1
	g.CurrentActivation = &Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 2}
	g.Units[0].Placed = true
	g.Units[0].X = 100
	g.Units[0].Y = 300
	g.Units[0].FacingDeg = 0
	g.Units[1].Placed = true
	g.Units[1].X = 0
	g.Units[1].Y = 200
	g.Units[1].FacingDeg = 0
	g.Units[1].Minis[0].WidthMM = 300
	g.Units[2].Placed = true
	g.Units[2].X = 100
	g.Units[2].Y = 100
	g.Units[2].FacingDeg = 180
	return g
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func hasCombatModifier(modifiers []CombatModifier, label string) bool {
	for _, modifier := range modifiers {
		if modifier.Label == label {
			return true
		}
	}
	return false
}
