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
			_, err := applyMove(&unit, act, ActionRequest{Type: ActionMove, Direction: "forward", DistanceMM: 20}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if mathRound(unit.X) != tc.wantX || mathRound(unit.Y) != tc.wantY {
				t.Fatalf("got (%v,%v), want (%v,%v)", unit.X, unit.Y, tc.wantX, tc.wantY)
			}
		})
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
	})
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
	})
	if err != nil {
		t.Fatal(err)
	}
	if moved != 50 || unit.Y != 50 {
		t.Fatalf("got moved %.0f y %.0f, want moved 50 y 50", moved, unit.Y)
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

func TestLegalActionsDoesNotExposeWheel(t *testing.T) {
	g := &Game{CurrentActivation: &Activation{UnitID: "u1"}}
	for _, action := range LegalActions(g) {
		if action == "wheel" {
			t.Fatal("wheel should not be a separate legal action")
		}
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
