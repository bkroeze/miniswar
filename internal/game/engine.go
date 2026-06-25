package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"
	"time"
)

type Engine struct {
	seed int64
	rng  *rand.Rand
}

func NewEngine(seed int64) *Engine {
	return &Engine{seed: seed, rng: rand.New(rand.NewSource(seed))}
}

func Battlemaps() []Battlemap {
	return []Battlemap{
		{
			ID:       "old_road",
			Name:     "Old Road",
			WidthMM:  ArenaWidthMM,
			HeightMM: ArenaHeightMM,
			Terrains: []TerrainZone{
				{ID: "old-road-east", Type: TerrainPath, Label: "road", Shape: "rect", X: 0, Y: 230, Width: 760, Height: 55},
				{ID: "old-road-north", Type: TerrainPath, Label: "road", Shape: "rect", X: 420, Y: 0, Width: 60, Height: 230},
				{ID: "marsh-west", Type: TerrainRough, Label: "rough", Shape: "rect", X: 250, Y: 70, Width: 175, Height: 130},
				{ID: "marsh-south", Type: TerrainRough, Label: "rough", Shape: "rect", X: 315, Y: 315, Width: 170, Height: 125},
			},
		},
		{
			ID:       "forest_wall",
			Name:     "Forest Wall",
			WidthMM:  ArenaWidthMM,
			HeightMM: ArenaHeightMM,
			Terrains: []TerrainZone{
				{ID: "lane-east", Type: TerrainPath, Label: "path", Shape: "rect", X: 575, Y: 0, Width: 55, Height: 520},
				{ID: "lane-cross", Type: TerrainPath, Label: "path", Shape: "rect", X: 120, Y: 210, Width: 510, Height: 45},
				{ID: "north-forest", Type: TerrainRough, Label: "forest", Shape: "rect", X: 270, Y: 55, Width: 165, Height: 125},
				{ID: "south-forest", Type: TerrainRough, Label: "forest", Shape: "rect", X: 455, Y: 315, Width: 110, Height: 120},
				{ID: "stone-wall", Type: TerrainImpassable, Label: "wall", Shape: "rect", X: 300, Y: 300, Width: 60, Height: 145},
				{ID: "low-wall", Type: TerrainImpassable, Label: "wall", Shape: "rect", X: 80, Y: 330, Width: 130, Height: 30},
			},
		},
	}
}

func BattlemapByID(id string) (Battlemap, bool) {
	maps := Battlemaps()
	if id == "" {
		return maps[0], true
	}
	for _, battlemap := range maps {
		if battlemap.ID == id {
			return battlemap, true
		}
	}
	return Battlemap{}, false
}

func NormalizeBattlemap(battlemap *Battlemap) {
	if battlemap == nil {
		return
	}
	if battlemap.WidthMM <= 0 {
		battlemap.WidthMM = ArenaWidthMM
	}
	if battlemap.HeightMM <= 0 {
		battlemap.HeightMM = ArenaHeightMM
	}
	if battlemap.Terrains == nil {
		battlemap.Terrains = []TerrainZone{}
	}
}

func ValidateBattlemap(battlemap Battlemap) error {
	NormalizeBattlemap(&battlemap)
	if strings.TrimSpace(battlemap.Name) == "" {
		return errors.New("battlemap name is required")
	}
	if battlemap.WidthMM <= 0 || battlemap.HeightMM <= 0 {
		return errors.New("battlemap dimensions must be greater than 0")
	}
	for _, terrain := range battlemap.Terrains {
		if err := validateTerrainZone(terrain, battlemap); err != nil {
			return err
		}
	}
	return nil
}

func validateTerrainZone(terrain TerrainZone, battlemap Battlemap) error {
	if strings.TrimSpace(terrain.ID) == "" {
		return errors.New("terrain id is required")
	}
	switch terrain.Type {
	case TerrainRough, TerrainImpassable, TerrainPath, TerrainPassableObstacle:
	default:
		return fmt.Errorf("unsupported terrain type %q", terrain.Type)
	}
	if terrain.Shape != "rect" {
		return fmt.Errorf("unsupported terrain shape %q", terrain.Shape)
	}
	if terrain.Width <= 0 || terrain.Height <= 0 {
		return fmt.Errorf("terrain %q dimensions must be greater than 0", terrain.ID)
	}
	if terrain.X < 0 || terrain.Y < 0 || terrain.X+terrain.Width > battlemap.WidthMM || terrain.Y+terrain.Height > battlemap.HeightMM {
		return fmt.Errorf("terrain %q must stay inside the battlemap", terrain.ID)
	}
	return nil
}

func Base(width, depth int) (BaseSize, bool) {
	switch {
	case width == 25 && depth == 25:
		return BaseSize{WidthMM: 25, DepthMM: 25, MaxMinis: 20, PerRank: 5}, true
	case width == 25 && depth == 50:
		return BaseSize{WidthMM: 25, DepthMM: 50, MaxMinis: 10, PerRank: 5}, true
	case width == 50 && depth == 50:
		return BaseSize{WidthMM: 50, DepthMM: 50, MaxMinis: 3, PerRank: 3}, true
	case width == 50 && depth == 100:
		return BaseSize{WidthMM: 50, DepthMM: 100, MaxMinis: 1, PerRank: 1}, true
	case width == 100 && depth == 50:
		return BaseSize{WidthMM: 100, DepthMM: 50, MaxMinis: 1, PerRank: 1}, true
	default:
		return BaseSize{}, false
	}
}

func (e *Engine) NewGame(setup Setup) (*Game, error) {
	battlemap := setup.Battlemap
	if battlemap.ID == "" && battlemap.Name == "" {
		var ok bool
		battlemap, ok = BattlemapByID(setup.BattlemapID)
		if !ok {
			return nil, fmt.Errorf("unknown battlemap %q", setup.BattlemapID)
		}
	}
	NormalizeBattlemap(&battlemap)
	if err := ValidateBattlemap(battlemap); err != nil {
		return nil, err
	}
	p1Setups := setup.Player1Units
	if len(p1Setups) == 0 {
		p1Setups = []UnitSetup{setup.Player1}
	}
	p2Setups := setup.Player2Units
	if len(p2Setups) == 0 {
		p2Setups = []UnitSetup{setup.Player2}
	}
	if len(p1Setups) == 0 || len(p2Setups) == 0 {
		return nil, errors.New("each player must have at least one unit")
	}
	units := make([]Unit, 0, len(p1Setups)+len(p2Setups))
	for i, unitSetup := range p1Setups {
		name := unitSetup.Name
		if name == "" {
			name = fmt.Sprintf("Player 1 Unit %d", i+1)
		}
		unit, err := newUnit(1, playerUnitID(1, i), name, unitSetup, 5, 0, 0, 0)
		if err != nil {
			return nil, fmt.Errorf("player1 unit %d: %w", i+1, err)
		}
		units = append(units, unit)
	}
	for i, unitSetup := range p2Setups {
		name := unitSetup.Name
		if name == "" {
			name = fmt.Sprintf("Player 2 Unit %d", i+1)
		}
		unit, err := newUnit(2, playerUnitID(2, i), name, unitSetup, 4, 0, 0, 180)
		if err != nil {
			return nil, fmt.Errorf("player2 unit %d: %w", i+1, err)
		}
		units = append(units, unit)
	}
	first := e.rng.Intn(2) + 1
	gameSeed := e.rng.Int63()
	g := &Game{
		ID:                  fmt.Sprintf("%d-%06d", time.Now().UnixNano(), e.rng.Intn(1000000)),
		Round:               1,
		ActivePlayer:        first,
		FirstPlayer:         first,
		Phase:               "setup",
		Units:               units,
		ActionHistory:       []ActionRecord{},
		Snapshots:           []SnapshotRecord{},
		CreatedAt:           time.Now().UTC(),
		RandomSeed:          gameSeed,
		RandomRollIndex:     0,
		OpeningInitiativeD2: first,
		Battlemap:           battlemap,
		Engagements:         []CombatEngagement{},
	}
	if allUnitsPlaced(g) {
		g.Phase = "awaiting_activation"
		completeGameIfWon(g)
	} else if !playerHasUnplacedUnit(g, g.ActivePlayer) {
		g.ActivePlayer = nextPlacementPlayer(g)
	}
	return g, nil
}

func playerUnitID(playerID, index int) string {
	if playerID == 1 && index == 0 {
		return "u1"
	}
	if playerID == 2 && index == 0 {
		return "u2"
	}
	return fmt.Sprintf("p%d-u%d", playerID, index+1)
}

func newUnit(player int, id, name string, setup UnitSetup, activation, x, y, facing int) (Unit, error) {
	base, ok := Base(setup.BaseWidthMM, setup.BaseDepthMM)
	if !ok {
		return Unit{}, errors.New("unsupported base size")
	}
	if setup.Count < 1 || setup.Count > base.MaxMinis {
		return Unit{}, fmt.Errorf("count must be between 1 and %d", base.MaxMinis)
	}
	if ((base.WidthMM == 50 && base.DepthMM == 100) || (base.WidthMM == 100 && base.DepthMM == 50)) && setup.Count != 1 {
		return Unit{}, errors.New("large artillery and monster bases must be a unit of 1")
	}
	unit := Unit{
		ID:               id,
		PlayerID:         player,
		Name:             name,
		CatalogUnitID:    setup.CatalogUnitID,
		ArmyID:           setup.ArmyID,
		ArmyUnitID:       setup.ArmyUnitID,
		MaxHealth:        setup.MaxHealth,
		CurrentHealth:    setup.CurrentHealth,
		CurrentHealthSet: setup.CurrentHealthSet,
		Stats:            setup.Stats,
		Special:          slices.Clone(setup.Special),
		Equipment:        slices.Clone(setup.Equipment),
		Base:             base,
		ActivationNumber: activation,
		MovementLimitMM:  movementLimitMM(setup.Stats),
		X:                float64(x),
		Y:                float64(y),
		FacingDeg:        normalizeDeg(facing),
	}
	unit.Minis = layoutMinis(unit, setup.Count)
	return unit, nil
}

func movementLimitMM(stats UnitStats) int {
	if stats.M > 0 {
		return stats.M * 25
	}
	return MovementLimitMM
}

func layoutMinis(unit Unit, count int) []Mini {
	minis := make([]Mini, 0, count)
	frontRankCount := min(count, unit.Base.PerRank)
	officerFile := max(0, (frontRankCount-1)/2)
	for i := 0; i < count; i++ {
		rank := i / unit.Base.PerRank
		file := i % unit.Base.PerRank
		health := miniStartingHealth(unit)
		minis = append(minis, Mini{
			Key:             fmt.Sprintf("p%d-%s-m%02d", unit.PlayerID, unit.ID, i+1),
			UnitID:          unit.ID,
			Index:           i + 1,
			Rank:            rank,
			File:            file,
			RelX:            float64(file * unit.Base.WidthMM),
			RelY:            float64(rank * unit.Base.DepthMM),
			WidthMM:         unit.Base.WidthMM,
			DepthMM:         unit.Base.DepthMM,
			IsOfficer:       rank == 0 && file == officerFile,
			HealthRemaining: health,
			Removed:         health <= 0,
		})
	}
	return minis
}

func cloneUnit(unit Unit) Unit {
	unit.Minis = slices.Clone(unit.Minis)
	return unit
}

func (e *Engine) PlaceUnit(g *Game, req PlacementRequest) (*ActionRecord, error) {
	if g.Phase != "setup" {
		return nil, errors.New("unit placement is already complete")
	}
	unitID, ok := placementUnitID(g)
	if !ok {
		return nil, errors.New("no unit remains to place")
	}
	if req.UnitID != unitID {
		return nil, fmt.Errorf("next unit to place is %s", unitID)
	}
	if req.PlayerID != g.ActivePlayer {
		return nil, fmt.Errorf("player %d is placing", g.ActivePlayer)
	}
	unit, ok := findUnit(g, req.UnitID)
	if !ok || unit.PlayerID != req.PlayerID {
		return nil, errors.New("unit does not belong to placing player")
	}
	if unit.Placed {
		return nil, errors.New("unit is already placed")
	}

	unit.FacingDeg = facingTowardBattlemapCenter(g.Battlemap, req.X, req.Y)
	if req.FacingDeg != nil {
		unit.FacingDeg = normalizeDeg(*req.FacingDeg)
	}
	officer, err := pivotAnchor(unit, "")
	if err != nil {
		return nil, err
	}
	relX, relY := rotatePoint(miniCenterX(officer), miniCenterY(officer), unit.FacingDeg)
	unit.X = req.X - relX
	unit.Y = req.Y - relY
	unit.Placed = true
	if unitOverlapsTerrain(*unit, unit.X, unit.Y, g.Battlemap.Terrains, TerrainImpassable) {
		unit.Placed = false
		return nil, errors.New("placement overlaps impassable terrain")
	}
	if unitOverlapsAnyUnit(*unit, unit.X, unit.Y, g.Units) {
		unit.Placed = false
		return nil, errors.New("placement overlaps another unit")
	}
	if !unitInsideBattlemap(*unit, unit.X, unit.Y, g.Battlemap) {
		unit.Placed = false
		return nil, errors.New("placement must keep the whole unit in the arena")
	}

	g.PlacementIndex++
	messages := []string{fmt.Sprintf("Placed %s facing %d degrees.", unit.Name, unit.FacingDeg)}
	if allUnitsPlaced(g) {
		g.Phase = "awaiting_activation"
		g.ActivePlayer = g.FirstPlayer
		messages = append(messages, fmt.Sprintf("Setup complete. Player %d starts round 1.", g.ActivePlayer))
		if message := completeGameIfWon(g); message != "" {
			messages = append(messages, message)
		}
	} else {
		g.ActivePlayer = nextPlacementPlayer(g)
	}
	rec := g.appendRecord(ActionPlace, req.PlayerID, req.UnitID, req, map[string]any{"unit": unit}, messages)
	return &rec, nil
}

func (e *Engine) Activate(g *Game, req ActivateRequest) (*ActionRecord, []int, error) {
	if g.PendingCombatChoice != nil {
		return nil, nil, errors.New("resolve the pending combat choice before activating")
	}
	if g.Phase == "complete" {
		return nil, nil, errors.New("game is complete")
	}
	if g.Phase == "setup" {
		return nil, nil, errors.New("finish unit placement before activating")
	}
	if g.CurrentActivation != nil {
		return nil, nil, errors.New("current unit still has actions remaining")
	}
	if req.PlayerID != g.ActivePlayer {
		return nil, nil, fmt.Errorf("player %d is active", g.ActivePlayer)
	}
	unit, ok := findUnit(g, req.UnitID)
	if !ok || unit.PlayerID != req.PlayerID {
		return nil, nil, errors.New("unit does not belong to active player")
	}
	if unit.Broken {
		return nil, nil, errors.New("broken units cannot activate")
	}
	if !unit.Placed || activeMiniCount(*unit) == 0 {
		return nil, nil, errors.New("unit is no longer on the battlefield")
	}
	if unitActivatedThisRound(g, req.UnitID) {
		return nil, nil, errors.New("unit has already activated this round")
	}
	target := unit.ActivationNumber
	if unit.Disordered {
		target++
	}
	roll := []int{rollD10(g), rollD10(g)}
	success := roll[0] >= target || roll[1] >= target
	actions := 1
	if success {
		actions = 2
	}
	wasDisordered := unit.Disordered
	if success && unit.Disordered {
		unit.Disordered = false
		actions = 1
	}
	g.CurrentActivation = &Activation{
		UnitID:                unit.ID,
		PlayerID:              unit.PlayerID,
		Success:               success,
		LimitedToSimpleAction: success && wasDisordered,
		ActionsRemaining:      actions,
		Roll:                  roll,
	}
	g.Phase = "activated"
	messages := []string{fmt.Sprintf("%s rolled %d and %d against activation %d", unit.Name, roll[0], roll[1], target)}
	if success {
		messages = append(messages, "Activation succeeded; two actions available.")
		if wasDisordered {
			messages = append(messages, "Disorder cleared; this activation is limited to one simple action.")
		}
	} else {
		messages = append(messages, "Activation failed; one simple action available.")
	}
	result := map[string]any{"success": success, "roll": roll, "target": target}
	if wasDisordered {
		result["wasDisordered"] = true
		result["disorderCleared"] = success
	}
	combatResults := e.resolveCombatsForUnit(g, req.UnitID, len(g.ActionHistory))
	if len(combatResults) > 0 {
		result["combatRounds"] = combatResults
		for _, combat := range combatResults {
			messages = append(messages, combatMessages(combat)...)
		}
		if g.PendingCombatChoice != nil {
			g.Phase = "pending_combat_choice"
		}
	}
	settleCurrentActivationAfterCombat(g, req.UnitID)
	if message := completeGameIfWon(g); message != "" {
		messages = append(messages, message)
	}
	rec := g.appendRecord(ActionActivate, req.PlayerID, req.UnitID, req, result, messages)
	return &rec, roll, nil
}

func (e *Engine) ApplyAction(g *Game, req ActionRequest) (*ActionRecord, error) {
	if g.Phase == "complete" {
		return nil, errors.New("game is complete")
	}
	if req.Type == ActionCombatPushback {
		return e.applyCombatChoice(g, req)
	}
	if g.PendingCombatChoice != nil {
		return nil, errors.New("resolve the pending combat choice before taking another action")
	}
	if g.Phase == "setup" {
		return nil, errors.New("finish unit placement before taking actions")
	}
	if g.CurrentActivation == nil {
		return nil, errors.New("activate a unit before taking actions")
	}
	act := g.CurrentActivation
	if req.PlayerID != act.PlayerID || req.UnitID != act.UnitID {
		return nil, errors.New("action must target the currently activated unit")
	}
	unit, ok := findUnit(g, req.UnitID)
	if !ok {
		return nil, errors.New("unit not found")
	}
	if unit.Broken || !unit.Placed || activeMiniCount(*unit) == 0 {
		return nil, errors.New("activated unit is no longer on the battlefield")
	}
	if act.ActionsRemaining < 1 {
		return nil, errors.New("no actions remaining")
	}

	var messages []string
	result := map[string]any{}
	switch req.Type {
	case ActionMove:
		moveResult, err := e.applyMove(g, unit, act, req, len(g.ActionHistory))
		if err != nil {
			return nil, err
		}
		result["movement"] = moveResult
		if moveResult.Status == "entered_combat" {
			messages = append(messages, fmt.Sprintf("Moved %s %.0fmm into combat with %s.", req.Direction, moveResult.DistanceMM, moveResult.DefenderUnitID))
			if moveResult.Combat != nil {
				result["combatRound"] = moveResult.Combat
				result["combatRounds"] = []CombatRoundResult{*moveResult.Combat}
				messages = append(messages, combatMessages(*moveResult.Combat)...)
			}
			if g.PendingCombatChoice != nil {
				g.Phase = "pending_combat_choice"
			}
			settleCurrentActivationAfterCombat(g, req.UnitID)
			if message := completeGameIfWon(g); message != "" {
				messages = append(messages, message)
			}
		} else if moveResult.Status == "blocked_combat_alignment" {
			messages = append(messages, fmt.Sprintf("Moved %s %.0fmm; combat alignment was blocked.", req.Direction, moveResult.DistanceMM))
		} else {
			messages = append(messages, fmt.Sprintf("Moved %s %.0fmm.", req.Direction, moveResult.DistanceMM))
		}
	case ActionPivot:
		anchor, err := pivotAnchor(unit, req.AnchorKey)
		if err != nil {
			return nil, err
		}
		applyPivot(unit, anchor, req.FacingDeg, g.Battlemap.Terrains, g.Units)
		messages = append(messages, fmt.Sprintf("Pivoted to %d degrees around %s.", unit.FacingDeg, anchor.Key))
	case ActionAboutFace:
		candidate := cloneUnit(*unit)
		if err := applyAboutFace(&candidate); err != nil {
			return nil, err
		}
		if err := validateUnitPosition(candidate, g.Battlemap, g.Units); err != nil {
			return nil, err
		}
		*unit = candidate
		messages = append(messages, "About face completed.")
	case ActionShoot:
		shootResult, err := e.applyShoot(g, unit, act, req)
		if err != nil {
			return nil, err
		}
		result["shooting"] = shootResult
		messages = append(messages, shootingMessages(shootResult)...)
		if message := completeGameIfWon(g); message != "" {
			messages = append(messages, message)
		}
	case ActionSkip:
		skipped := act.ActionsRemaining
		act.ActionsRemaining = 1
		messages = append(messages, fmt.Sprintf("Skipped %d remaining action(s).", skipped))
	default:
		return nil, errors.New("unsupported phase 1 action")
	}

	if g.CurrentActivation != nil {
		act.ActionsRemaining--
		if req.Type == ActionMove {
			act.MovesTaken++
		}
		if req.Type == ActionShoot {
			act.ShotsTaken++
		}
		if act.ActionsRemaining == 0 && g.PendingCombatChoice == nil {
			g.CurrentActivation = nil
			g.Phase = "awaiting_activation"
			advanceTurn(g)
		}
	}
	if len(result) == 0 {
		result["unit"] = unit
	}
	rec := g.appendRecord(req.Type, req.PlayerID, req.UnitID, req, result, messages)
	return &rec, nil
}

func (e *Engine) applyCombatChoice(g *Game, req ActionRequest) (*ActionRecord, error) {
	choice := g.PendingCombatChoice
	if choice == nil {
		return nil, errors.New("no pending combat choice")
	}
	if req.PlayerID != choice.WinningPlayerID {
		return nil, errors.New("pending combat choice belongs to the combat winner")
	}
	if req.UnitID != choice.WinningUnitID {
		return nil, errors.New("combat choice must be submitted by the winning unit")
	}
	if !slices.Contains(choice.Choices, req.CombatChoice) {
		return nil, errors.New("invalid combat choice")
	}
	messages := []string{}
	result := map[string]any{"choice": req.CombatChoice, "pendingChoice": choice}
	switch req.CombatChoice {
	case CombatChoicePushback150:
		dx, dy := pushbackVector(g, choice)
		choiceResult := moveCombatChoiceUnit(g, choice.LosingUnitID, dx, dy, 150, choice.WinningUnitID)
		choiceResult.Choice = req.CombatChoice
		result["combatChoice"] = choiceResult
		result["distanceMovedMm"] = choiceResult.MovedDistanceMM
		messages = append(messages, fmt.Sprintf("Pushed %s %.0fmm.", choice.LosingUnitID, choiceResult.MovedDistanceMM))
	case CombatChoiceWithdraw25:
		dx, dy := withdrawVector(g, choice)
		choiceResult := moveCombatChoiceUnit(g, choice.WinningUnitID, dx, dy, 25, choice.LosingUnitID)
		choiceResult.Choice = req.CombatChoice
		result["combatChoice"] = choiceResult
		result["distanceMovedMm"] = choiceResult.MovedDistanceMM
		messages = append(messages, fmt.Sprintf("Withdrew %s %.0fmm.", choice.WinningUnitID, choiceResult.MovedDistanceMM))
	case CombatChoiceDecline:
		result["combatChoice"] = CombatChoiceResult{Choice: req.CombatChoice}
		messages = append(messages, "Combat winner declined pushback or withdraw.")
	}
	deactivateEngagement(g, choice.EngagementID)
	g.PendingCombatChoice = nil
	if g.CurrentActivation != nil {
		if g.CurrentActivation.ActionsRemaining <= 0 {
			g.CurrentActivation = nil
			g.Phase = "awaiting_activation"
			advanceTurn(g)
		} else {
			g.Phase = "activated"
		}
	} else {
		g.Phase = "awaiting_activation"
	}
	if message := completeGameIfWon(g); message != "" {
		messages = append(messages, message)
	}
	rec := g.appendRecord(ActionCombatPushback, req.PlayerID, choice.WinningUnitID, req, result, messages)
	return &rec, nil
}

func moveCombatChoiceUnit(g *Game, unitID string, dx, dy, distance float64, ignoredUnitIDs ...string) CombatChoiceResult {
	result := CombatChoiceResult{
		MovingUnitID:        unitID,
		RequestedDistanceMM: distance,
		AxisDX:              dx,
		AxisDY:              dy,
		StoppedBy:           "completed",
	}
	unit, ok := findUnit(g, unitID)
	if !ok {
		result.StoppedBy = "missing_unit"
		return result
	}
	ignored := map[string]bool{}
	for _, ignoredUnitID := range ignoredUnitIDs {
		if ignoredUnitID != "" && movingAwayFromUnit(g, *unit, ignoredUnitID, dx, dy) {
			ignored[ignoredUnitID] = true
		}
	}
	result.Start = Position{X: unit.X, Y: unit.Y}
	moved := 0.0
	for moved < distance {
		step := minFloat(1, distance-moved)
		snapshot := unitPositionSnapshot(g.Units)
		nextX := unit.X + dx*step
		nextY := unit.Y + dy*step
		if !pushbackPositionValid(*unit, nextX, nextY, g) {
			result.StoppedBy = "obstacle_or_arena"
			break
		}
		unit.X = nextX
		unit.Y = nextY
		if !resolvePushbackChain(g, unit.ID, dx, dy, ignored) {
			restoreUnitPositionSnapshot(g.Units, snapshot)
			unit, _ = findUnit(g, unitID)
			result.StoppedBy = "unit"
			break
		}
		moved += step
	}
	result.MovedDistanceMM = moved
	result.End = Position{X: unit.X, Y: unit.Y}
	return result
}

func unitPositionSnapshot(units []Unit) map[string]Position {
	snapshot := make(map[string]Position, len(units))
	for _, unit := range units {
		snapshot[unit.ID] = Position{X: unit.X, Y: unit.Y}
	}
	return snapshot
}

func restoreUnitPositionSnapshot(units []Unit, snapshot map[string]Position) {
	for i := range units {
		if position, ok := snapshot[units[i].ID]; ok {
			units[i].X = position.X
			units[i].Y = position.Y
		}
	}
}

func movingAwayFromUnit(g *Game, moving Unit, otherUnitID string, dx, dy float64) bool {
	other, ok := findUnit(g, otherUnitID)
	if !ok {
		return false
	}
	movingX, movingY := unitCenter(moving)
	otherX, otherY := unitCenter(*other)
	return (movingX-otherX)*dx+(movingY-otherY)*dy > 0
}

func resolvePushbackChain(g *Game, movedUnitID string, dx, dy float64, ignored map[string]bool) bool {
	queue := []string{movedUnitID}
	queued := map[string]bool{movedUnitID: true}
	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]
		current, ok := findUnit(g, currentID)
		if !ok {
			continue
		}
		for i := range g.Units {
			other := &g.Units[i]
			if other.ID == current.ID || ignored[other.ID] || !other.Placed || other.Broken || activeMiniCount(*other) == 0 {
				continue
			}
			distance := unitDistance(*current, *other)
			if distance >= 25 {
				continue
			}
			if !moveChainedUnitAway(g, other, *current, dx, dy, 25-distance) {
				return false
			}
			if !queued[other.ID] {
				queued[other.ID] = true
				queue = append(queue, other.ID)
			}
		}
	}
	return true
}

func moveChainedUnitAway(g *Game, unit *Unit, source Unit, dx, dy, minDistance float64) bool {
	moved := 0.0
	for unitDistance(*unit, source) < 25 {
		if moved > minDistance+200 {
			return false
		}
		step := 1.0
		nextX := unit.X + dx*step
		nextY := unit.Y + dy*step
		if !pushbackPositionValid(*unit, nextX, nextY, g) {
			return false
		}
		unit.X = nextX
		unit.Y = nextY
		moved += step
	}
	return true
}

func pushbackPositionValid(unit Unit, x, y float64, g *Game) bool {
	return unitInsideBattlemap(unit, x, y, g.Battlemap) && !unitOverlapsTerrain(unit, x, y, g.Battlemap.Terrains, TerrainImpassable)
}

func unitDistance(a, b Unit) float64 {
	distance := math.Inf(1)
	for _, mini := range a.Minis {
		if mini.Removed {
			continue
		}
		aPoly := miniWorldPolygon(a, mini, a.X, a.Y)
		for _, otherMini := range b.Minis {
			if otherMini.Removed {
				continue
			}
			distance = math.Min(distance, polygonDistance(aPoly, miniWorldPolygon(b, otherMini, b.X, b.Y)))
		}
	}
	if math.IsInf(distance, 1) {
		return 0
	}
	return distance
}

func pushbackVector(g *Game, choice *PendingCombatChoice) (float64, float64) {
	if winner, ok := findUnit(g, choice.WinningUnitID); ok {
		return facingVector(winner.FacingDeg, 1)
	}
	if choice.WinningIsAttacker {
		return choice.AxisDX, choice.AxisDY
	}
	return -choice.AxisDX, -choice.AxisDY
}

func withdrawVector(g *Game, choice *PendingCombatChoice) (float64, float64) {
	dx, dy := pushbackVector(g, choice)
	return -dx, -dy
}

func settleCurrentActivationAfterCombat(g *Game, unitID string) {
	if g.PendingCombatChoice != nil || g.CurrentActivation == nil || g.CurrentActivation.UnitID != unitID {
		return
	}
	unit, ok := findUnit(g, unitID)
	if ok && unit.Placed && !unit.Broken && activeMiniCount(*unit) > 0 {
		return
	}
	g.CurrentActivation = nil
	g.Phase = "awaiting_activation"
	advanceTurn(g)
}

func applyMove(unit *Unit, act *Activation, req ActionRequest, terrains []TerrainZone, units []Unit) (float64, error) {
	if req.Direction != "forward" && req.Direction != "backward" {
		return 0, errors.New("move direction must be forward or backward")
	}
	limit := float64(unit.MovementLimitMM)
	if req.Direction == "backward" {
		limit = limit / 2
	}
	if act.MovesTaken > 0 {
		limit = limit / 2
	}
	if req.DistanceMM <= 0 || req.DistanceMM > limit {
		return 0, fmt.Errorf("distance must be greater than 0 and no more than %.0fmm", limit)
	}
	sign := 1.0
	if req.Direction == "backward" {
		sign = -1
	}
	rad := float64(unit.FacingDeg) * math.Pi / 180
	dx := math.Sin(rad) * sign
	dy := -math.Cos(rad) * sign
	startX := unit.X
	startY := unit.Y
	remaining := limit
	moved := 0.0
	for moved < req.DistanceMM && remaining > 0 {
		step := minFloat(1, req.DistanceMM-moved)
		nextX := unit.X + dx*step
		nextY := unit.Y + dy*step
		if unitOverlapsTerrain(*unit, nextX, nextY, terrains, TerrainImpassable) || unitOverlapsEnemyUnit(*unit, nextX, nextY, units) {
			break
		}
		cost := step
		if unitOverlapsTerrain(*unit, nextX, nextY, terrains, TerrainRough) {
			cost *= 2
		}
		if cost > remaining+0.000001 {
			break
		}
		unit.X = nextX
		unit.Y = nextY
		moved += step
		remaining -= cost
	}
	if unitOverlapsFriendlyUnit(*unit, unit.X, unit.Y, units) {
		moved = revertToBeforeFriendlyOverlap(unit, req, terrains, units, startX, startY, moved)
	}
	if unitOverlapsAnyUnit(*unit, unit.X, unit.Y, units) {
		unit.X = startX
		unit.Y = startY
		return 0, nil
	}
	return moved, nil
}

func (e *Engine) applyMove(g *Game, unit *Unit, act *Activation, req ActionRequest, actionIndex int) (MoveResult, error) {
	if req.Direction != "forward" && req.Direction != "backward" {
		return MoveResult{}, errors.New("move direction must be forward or backward")
	}
	limit := float64(unit.MovementLimitMM)
	if req.Direction == "backward" {
		limit = limit / 2
	}
	if act.MovesTaken > 0 {
		limit = limit / 2
	}
	if req.DistanceMM <= 0 || req.DistanceMM > limit {
		return MoveResult{}, fmt.Errorf("distance must be greater than 0 and no more than %.0fmm", limit)
	}
	sign := 1.0
	if req.Direction == "backward" {
		sign = -1
	}
	dx, dy := facingVector(unit.FacingDeg, sign)
	startX := unit.X
	startY := unit.Y
	remaining := limit
	moved := 0.0
	for moved < req.DistanceMM && remaining > 0 {
		step := minFloat(1, req.DistanceMM-moved)
		nextX := unit.X + dx*step
		nextY := unit.Y + dy*step
		if !unitInsideBattlemap(*unit, nextX, nextY, g.Battlemap) || unitOverlapsTerrain(*unit, nextX, nextY, g.Battlemap.Terrains, TerrainImpassable) {
			break
		}
		if enemy := firstContactingEnemy(*unit, nextX, nextY, g.Units); enemy != nil {
			face := contactedFace(*enemy, *unit, nextX, nextY)
			defenderFortified := movedIntoCombatAcrossPassableObstacle(*unit, startX, startY, nextX, nextY, *enemy, g.Battlemap.Terrains)
			if !snapAttackerFlush(unit, *enemy, face, g.Battlemap, g.Units) {
				return MoveResult{Status: "blocked_combat_alignment", DistanceMM: moved, DefenderUnitID: enemy.ID, DefenderFace: face}, nil
			}
			engagement := CombatEngagement{
				ID:                 fmt.Sprintf("combat-%d", actionIndex),
				AttackerUnitID:     unit.ID,
				DefenderUnitID:     enemy.ID,
				DefenderFace:       face,
				DefenderFortified:  defenderFortified,
				AxisDX:             dx,
				AxisDY:             dy,
				Round:              g.Round,
				CreatedActionIndex: actionIndex,
				Active:             true,
			}
			g.Engagements = append(g.Engagements, engagement)
			combat := e.resolveCombatRound(g, engagement, actionIndex, unit.ID, nil)
			return MoveResult{
				Status:         "entered_combat",
				DistanceMM:     moved + step,
				DefenderUnitID: enemy.ID,
				DefenderFace:   face,
				EngagementID:   engagement.ID,
				Combat:         &combat,
			}, nil
		}
		cost := step
		if unitOverlapsTerrain(*unit, nextX, nextY, g.Battlemap.Terrains, TerrainRough) {
			cost *= 2
		}
		if cost > remaining+0.000001 {
			break
		}
		unit.X = nextX
		unit.Y = nextY
		moved += step
		remaining -= cost
	}
	if unitOverlapsFriendlyUnit(*unit, unit.X, unit.Y, g.Units) {
		moved = revertToBeforeFriendlyOverlap(unit, req, g.Battlemap.Terrains, g.Units, startX, startY, moved)
	}
	if unitOverlapsAnyUnit(*unit, unit.X, unit.Y, g.Units) {
		unit.X = startX
		unit.Y = startY
		return MoveResult{Status: "blocked", DistanceMM: 0}, nil
	}
	return MoveResult{Status: "moved", DistanceMM: moved}, nil
}

var shootingWeaponRanges = map[string]int{
	"Bow":            20,
	"Elf Bow":        22,
	"Sling":          12,
	"Light Catapult": 32,
	"Heavy Catapult": 40,
	"Ballista":       30,
	"Fire Breath":    12,
}

func (e *Engine) applyShoot(g *Game, unit *Unit, act *Activation, req ActionRequest) (ShootResult, error) {
	if !act.Success {
		return ShootResult{}, errors.New("failed activations may only take simple actions")
	}
	if req.TargetUnitID == "" {
		return ShootResult{}, errors.New("shoot action requires a target unit id")
	}
	target, ok := findUnit(g, req.TargetUnitID)
	if !ok {
		return ShootResult{}, errors.New("target unit not found")
	}
	if target.PlayerID == unit.PlayerID {
		return ShootResult{}, errors.New("shooting target must be an enemy unit")
	}
	if target.Broken || !target.Placed || activeMiniCount(*target) == 0 {
		return ShootResult{}, errors.New("shooting target is no longer on the battlefield")
	}
	detail, err := legalShootTarget(g, *unit, *target, act)
	if err != nil {
		return ShootResult{}, err
	}

	targetNumber, modifiers := shootingTargetNumber(g, *unit, *target, act)
	dice := shootingDiceCount(*unit, *target)
	rolls := make([]int, 0, dice)
	hits := 0
	for i := 0; i < dice; i++ {
		roll := rollD10(g)
		rolls = append(rolls, roll)
		hits += hitsForRoll(roll, targetNumber)
	}
	hitResult := applyHitsToUnit(target, hits)
	removeUnitIfNoActiveMinis(target)
	moraleTested := moraleTestedThisRound(g)
	var moraleTests []MoraleTestResult
	if moraleRequired(*target, hitResult) && activeMiniCount(*target) > 0 {
		if morale, ok := e.resolveShootingMoraleOnce(g, target, moraleTested); ok {
			moraleTests = append(moraleTests, morale)
		}
	}
	if target.Broken {
		moraleTests = append(moraleTests, e.resolveBrokenCascade(g, target.ID, moraleTested)...)
	}
	brokenUnits := brokenUnitsFromMorale(moraleTests)
	if target.Broken {
		brokenUnits = appendUniqueString(brokenUnits, target.ID)
	}
	return ShootResult{
		TargetUnitID:  target.ID,
		Weapon:        detail.Weapon,
		RangeMM:       detail.RangeMM,
		RangeLimitMM:  detail.RangeLimitMM,
		LOS:           detail.LOS,
		DiceCount:     dice,
		TargetNumber:  targetNumber,
		Modifiers:     modifiers,
		Rolls:         rolls,
		Hits:          hits,
		Casualties:    hitResult.Casualties,
		MoraleTests:   moraleTests,
		BrokenUnits:   brokenUnits,
		TargetRemoved: target.Broken || !target.Placed || activeMiniCount(*target) == 0,
	}, nil
}

func legalShootTarget(g *Game, attacker, target Unit, act *Activation) (ShootTarget, error) {
	weapon, rangeMM, ok := shootingWeapon(attacker)
	if !ok {
		return ShootTarget{}, errors.New("unit has no shooting weapon")
	}
	if act == nil || act.UnitID != attacker.ID || act.PlayerID != attacker.PlayerID {
		return ShootTarget{}, errors.New("shooting requires the current activation")
	}
	if act.ActionsRemaining < 1 {
		return ShootTarget{}, errors.New("no actions remaining")
	}
	if act.ShotsTaken > 0 {
		return ShootTarget{}, errors.New("unit has already shot this activation")
	}
	if !act.Success {
		return ShootTarget{}, errors.New("failed activations may only take simple actions")
	}
	if act.LimitedToSimpleAction {
		return ShootTarget{}, errors.New("disorder-clearing activations may only take simple actions")
	}
	if attacker.Broken || !attacker.Placed || activeMiniCount(attacker) == 0 {
		return ShootTarget{}, errors.New("shooting unit is no longer on the battlefield")
	}
	if unitInActiveCombat(g, attacker.ID) {
		return ShootTarget{}, errors.New("units in combat cannot shoot")
	}
	if unitInActiveCombat(g, target.ID) {
		return ShootTarget{}, errors.New("units in combat cannot be targeted by shooting")
	}
	distance := shootingRangeMM(attacker, target)
	if distance > rangeMM+0.000001 {
		return ShootTarget{}, fmt.Errorf("target is out of range: %.0fmm > %.0fmm", distance, rangeMM)
	}
	los := lineOfSight(g, attacker, target)
	if !los.OK {
		return ShootTarget{}, errors.New("target is not in line of sight")
	}
	cx, cy := unitCenter(target)
	return ShootTarget{
		UnitID:       target.ID,
		Weapon:       weapon,
		RangeMM:      distance,
		RangeLimitMM: rangeMM,
		LOS:          los,
		Center:       Position{X: cx, Y: cy},
	}, nil
}

func LegalActionDetails(g *Game) []LegalAction {
	actions := LegalActions(g)
	if len(actions) == 0 {
		return nil
	}
	details := make([]LegalAction, 0, len(actions))
	for _, action := range actions {
		detail := LegalAction{Type: action}
		if action == ActionShoot {
			detail.Targets = legalShootTargets(g)
		}
		details = append(details, detail)
	}
	return details
}

func legalShootTargets(g *Game) []ShootTarget {
	if g == nil || g.CurrentActivation == nil {
		return nil
	}
	attacker, ok := findUnit(g, g.CurrentActivation.UnitID)
	if !ok {
		return nil
	}
	targets := []ShootTarget{}
	for _, target := range g.Units {
		if target.PlayerID == attacker.PlayerID || target.Broken || !target.Placed || activeMiniCount(target) == 0 {
			continue
		}
		detail, err := legalShootTarget(g, *attacker, target, g.CurrentActivation)
		if err == nil {
			targets = append(targets, detail)
		}
	}
	return targets
}

func shootingWeapon(unit Unit) (string, float64, bool) {
	fields := append([]string{}, unit.Equipment...)
	fields = append(fields, unit.Special...)
	fields = append(fields, unit.Name)
	bestName := ""
	bestRange := 0
	for weapon, rangeUnits := range shootingWeaponRanges {
		for _, field := range fields {
			if strings.Contains(normalizeRuleToken(field), normalizeRuleToken(weapon)) {
				if rangeUnits > bestRange {
					bestName = weapon
					bestRange = rangeUnits
				}
			}
		}
	}
	if bestName == "" {
		return "", 0, false
	}
	return bestName, float64(bestRange * 25), true
}

func hasSpecialAbility(unit Unit, ability string) bool {
	needle := normalizeRuleToken(ability)
	for _, special := range unit.Special {
		if strings.Contains(normalizeRuleToken(special), needle) {
			return true
		}
	}
	return false
}

func normalizeRuleToken(value string) string {
	value = strings.ToLower(value)
	replacer := strings.NewReplacer("-", " ", "_", " ", "(", " ", ")", " ", ",", " ", ":", " ")
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func unitInActiveCombat(g *Game, unitID string) bool {
	for _, engagement := range g.Engagements {
		if !engagement.Active {
			continue
		}
		if engagement.AttackerUnitID == unitID || engagement.DefenderUnitID == unitID {
			return true
		}
	}
	return false
}

func shootingRangeMM(attacker, target Unit) float64 {
	officer, err := pivotAnchor(&attacker, "")
	if err != nil {
		ax, ay := unitCenter(attacker)
		tx, ty := unitCenter(target)
		return math.Hypot(tx-ax, ty-ay)
	}
	attackerPoints := miniPolygonPoints(miniWorldPolygon(attacker, officer, attacker.X, attacker.Y))
	distance := math.Inf(1)
	for _, targetMini := range target.Minis {
		if targetMini.Removed || miniHealth(target, targetMini) <= 0 {
			continue
		}
		for _, a := range attackerPoints {
			for _, b := range miniPolygonPoints(miniWorldPolygon(target, targetMini, target.X, target.Y)) {
				distance = math.Min(distance, math.Hypot(b[0]-a[0], b[1]-a[1]))
			}
		}
	}
	if math.IsInf(distance, 1) {
		return 0
	}
	return distance
}

func shootingDiceCount(attacker, target Unit) int {
	dice := attacker.Stats.CD * activeFrontRankCount(attacker)
	if hasSpecialAbility(target, "Shielding") {
		dice--
	}
	if dice < 1 {
		return 1
	}
	return dice
}

func shootingTargetNumber(g *Game, attacker, defender Unit, act *Activation) (int, []CombatModifier) {
	target := defender.Stats.D - attacker.Stats.A
	var modifiers []CombatModifier
	add := func(label string, value int) {
		if value == 0 {
			return
		}
		modifiers = append(modifiers, CombatModifier{Label: label, Value: value})
		target += value
	}
	add("shooting ranks", -max(0, activeFullRanks(attacker)-1))
	if act != nil && act.ActionsRemaining == 1 {
		add("second action of activation", 1)
	}
	if attacker.Disordered {
		add("shooter disordered", 1)
	}
	add("target light cover", shootingCoverModifier(g, defender, "light"))
	add("target heavy cover", shootingCoverModifier(g, defender, "heavy"))
	return target, modifiers
}

func shootingCoverModifier(g *Game, defender Unit, cover string) int {
	_ = g
	_ = defender
	_ = cover
	return 0
}

func (e *Engine) resolveShootingMoraleOnce(g *Game, unit *Unit, tested map[string]bool) (MoraleTestResult, bool) {
	if tested[unit.ID] {
		return MoraleTestResult{}, false
	}
	target, modifiers := moraleTargetNumber(*unit, false, true)
	rolls := []int{rollD10(g), rollD10(g)}
	passed := rolls[0] >= target || rolls[1] >= target
	outcome := "passed"
	wasDisordered := unit.Disordered
	if !passed {
		if wasDisordered {
			unit.Broken = true
			unit.Placed = false
			deactivateEngagementsForUnit(g, unit.ID)
			outcome = UnitStatusBroken
		} else {
			unit.Disordered = true
			outcome = UnitStatusDisordered
		}
	}
	tested[unit.ID] = true
	return MoraleTestResult{
		UnitID:       unit.ID,
		Rolls:        rolls,
		TargetNumber: target,
		Modifiers:    modifiers,
		Passed:       passed,
		Cascade:      false,
		Outcome:      outcome,
	}, true
}

func shootingMessages(result ShootResult) []string {
	messages := []string{
		fmt.Sprintf("Shot %s with %s at %.0fmm: rolled %v vs TN %d for %d hit(s).",
			result.TargetUnitID, result.Weapon, result.RangeMM, result.Rolls, result.TargetNumber, result.Hits),
	}
	for _, casualty := range result.Casualties {
		if casualty.Removed {
			messages = append(messages, fmt.Sprintf("%s removed from %s.", casualty.MiniKey, casualty.UnitID))
		}
	}
	for _, morale := range result.MoraleTests {
		messages = append(messages, fmt.Sprintf("%s shooting morale rolled %v against %d: %s.", morale.UnitID, morale.Rolls, morale.TargetNumber, morale.Outcome))
	}
	return messages
}

func lineOfSight(g *Game, attacker, target Unit) LineOfSightResult {
	if hasSpecialAbility(attacker, "Indirect Fire") {
		return LineOfSightResult{OK: true, TargetFacing: "indirect", Indirect: true}
	}
	officer, err := pivotAnchor(&attacker, "")
	if err != nil {
		return LineOfSightResult{OK: false, BlockedBy: "no_officer"}
	}
	if activeMiniCount(target) == 1 {
		for _, mini := range target.Minis {
			if mini.Removed || miniHealth(target, mini) <= 0 {
				continue
			}
			if canSeeMini(g, attacker, officer, target, mini, "") {
				return LineOfSightResult{OK: true, TargetFacing: "single", RequiredFigures: 1, VisibleFigures: 1, VisibleMiniKeys: []string{mini.Key}}
			}
			return LineOfSightResult{OK: false, TargetFacing: "single", RequiredFigures: 1}
		}
	}
	best := LineOfSightResult{}
	for _, face := range []string{CombatFaceFront, CombatFaceRight, CombatFaceRear, CombatFaceLeft} {
		figures := targetFacingMinis(target, face)
		if len(figures) == 0 {
			continue
		}
		required := (len(figures) + 1) / 2
		result := LineOfSightResult{TargetFacing: face, RequiredFigures: required}
		for _, mini := range figures {
			if canSeeMini(g, attacker, officer, target, mini, face) {
				result.VisibleFigures++
				result.VisibleMiniKeys = append(result.VisibleMiniKeys, mini.Key)
			}
		}
		if result.VisibleFigures > best.VisibleFigures {
			best = result
		}
		if result.VisibleFigures >= required {
			result.OK = true
			return result
		}
	}
	if best.TargetFacing == "" {
		best.BlockedBy = "no_target_figures"
	} else {
		best.BlockedBy = "blocked_or_out_of_arc"
	}
	return best
}

func canSeeMini(g *Game, attacker Unit, officer Mini, target Unit, targetMini Mini, targetFace string) bool {
	fromPoints := miniPolygonPoints(miniWorldPolygon(attacker, officer, attacker.X, attacker.Y))
	toPoints := targetMiniSightPoints(target, targetMini, targetFace)
	for _, from := range fromPoints {
		for _, to := range toPoints {
			if !pointInFrontArc(attacker, to[0], to[1]) {
				continue
			}
			if !lineBlocked(g, attacker, target, targetMini.Key, from[0], from[1], to[0], to[1]) {
				return true
			}
		}
	}
	return false
}

func miniPolygonPoints(poly [4][2]float64) [][2]float64 {
	cx := 0.0
	cy := 0.0
	points := make([][2]float64, 0, 5)
	for _, point := range poly {
		points = append(points, point)
		cx += point[0]
		cy += point[1]
	}
	points = append(points, [2]float64{cx / 4, cy / 4})
	return points
}

func targetMiniSightPoints(unit Unit, mini Mini, face string) [][2]float64 {
	poly := miniWorldPolygon(unit, mini, unit.X, unit.Y)
	if face == "" {
		return miniPolygonPoints(poly)
	}
	var local [][2]float64
	switch face {
	case CombatFaceFront:
		local = [][2]float64{{mini.RelX, mini.RelY}, {mini.RelX + float64(mini.WidthMM)/2, mini.RelY}, {mini.RelX + float64(mini.WidthMM), mini.RelY}}
	case CombatFaceRear:
		y := mini.RelY + float64(mini.DepthMM)
		local = [][2]float64{{mini.RelX, y}, {mini.RelX + float64(mini.WidthMM)/2, y}, {mini.RelX + float64(mini.WidthMM), y}}
	case CombatFaceRight:
		x := mini.RelX + float64(mini.WidthMM)
		local = [][2]float64{{x, mini.RelY}, {x, mini.RelY + float64(mini.DepthMM)/2}, {x, mini.RelY + float64(mini.DepthMM)}}
	case CombatFaceLeft:
		local = [][2]float64{{mini.RelX, mini.RelY}, {mini.RelX, mini.RelY + float64(mini.DepthMM)/2}, {mini.RelX, mini.RelY + float64(mini.DepthMM)}}
	default:
		return miniPolygonPoints(poly)
	}
	points := make([][2]float64, 0, len(local))
	for _, point := range local {
		x, y := rotatePoint(point[0], point[1], unit.FacingDeg)
		points = append(points, [2]float64{unit.X + x, unit.Y + y})
	}
	return points
}

func pointInFrontArc(unit Unit, x, y float64) bool {
	officer, err := pivotAnchor(&unit, "")
	if err != nil {
		return false
	}
	ox, oy := miniWorldCenter(unit, officer, unit.FacingDeg)
	dx := x - ox
	dy := y - oy
	if math.Hypot(dx, dy) <= 0.000001 {
		return true
	}
	targetDeg := normalizeDeg(int(math.Round(math.Atan2(dx, -dy) * 180 / math.Pi)))
	return angleDifferenceDeg(unit.FacingDeg, targetDeg) <= 90
}

func angleDifferenceDeg(a, b int) int {
	diff := math.Abs(float64(((a-b)%360+540)%360 - 180))
	return int(diff)
}

func lineBlocked(g *Game, attacker, target Unit, targetMiniKey string, ax, ay, bx, by float64) bool {
	for _, terrain := range g.Battlemap.Terrains {
		if terrainBlocksLineOfSight(terrain, target, ax, ay, bx, by) {
			return true
		}
	}
	for _, unit := range g.Units {
		if unit.ID == attacker.ID || unit.ID == target.ID || !unit.Placed || unit.Broken || activeMiniCount(unit) == 0 {
			continue
		}
		if !blocksLineOfSightTo(unit, target) {
			continue
		}
		for _, mini := range unit.Minis {
			if mini.Removed || miniHealth(unit, mini) <= 0 {
				continue
			}
			if segmentIntersectsRect(ax, ay, bx, by, polygonBounds(miniWorldPolygon(unit, mini, unit.X, unit.Y))) {
				return true
			}
		}
	}
	_ = targetMiniKey
	return false
}

func terrainBlocksLineOfSight(terrain TerrainZone, target Unit, ax, ay, bx, by float64) bool {
	_ = terrain
	_ = target
	_ = ax
	_ = ay
	_ = bx
	_ = by
	return false
}

func blocksLineOfSightTo(blockingUnit, targetUnit Unit) bool {
	if hasSpecialAbility(targetUnit, "Enormous") {
		return hasSpecialAbility(blockingUnit, "Enormous")
	}
	if hasSpecialAbility(targetUnit, "Large") {
		return hasSpecialAbility(blockingUnit, "Large")
	}
	return true
}

func targetFacingMinis(unit Unit, face string) []Mini {
	bounds := activeLocalBounds(unit)
	var out []Mini
	for _, mini := range unit.Minis {
		if mini.Removed || miniHealth(unit, mini) <= 0 {
			continue
		}
		switch face {
		case CombatFaceFront:
			if math.Abs(mini.RelY-bounds.minY) <= 0.000001 {
				out = append(out, mini)
			}
		case CombatFaceRear:
			if math.Abs(mini.RelY+float64(mini.DepthMM)-bounds.maxY) <= 0.000001 {
				out = append(out, mini)
			}
		case CombatFaceRight:
			if math.Abs(mini.RelX+float64(mini.WidthMM)-bounds.maxX) <= 0.000001 {
				out = append(out, mini)
			}
		case CombatFaceLeft:
			if math.Abs(mini.RelX-bounds.minX) <= 0.000001 {
				out = append(out, mini)
			}
		}
	}
	return out
}

func facingVector(facingDeg int, sign float64) (float64, float64) {
	rad := float64(facingDeg) * math.Pi / 180
	return math.Sin(rad) * sign, -math.Cos(rad) * sign
}

func firstOverlappingEnemy(unit Unit, x, y float64, units []Unit) *Unit {
	for i := range units {
		other := &units[i]
		if other.ID == unit.ID || !other.Placed || other.Broken || other.PlayerID == unit.PlayerID {
			continue
		}
		if unitsMiniRectsOverlap(unit, x, y, *other) {
			return other
		}
	}
	return nil
}

func firstContactingEnemy(unit Unit, x, y float64, units []Unit) *Unit {
	for i := range units {
		other := &units[i]
		if other.ID == unit.ID || !other.Placed || other.Broken || other.PlayerID == unit.PlayerID || activeMiniCount(*other) == 0 {
			continue
		}
		if unitsMiniRectsOverlap(unit, x, y, *other) || unitsMiniRectsTouch(unit, x, y, *other) {
			return other
		}
	}
	return nil
}

func movedIntoCombatAcrossPassableObstacle(attacker Unit, startX, startY, contactX, contactY float64, defender Unit, terrains []TerrainZone) bool {
	startBox := unitBoundsAt(attacker, startX, startY)
	contactBox := unitBoundsAt(attacker, contactX, contactY)
	defenderBox := unitBoundsAt(defender, defender.X, defender.Y)
	startCX, startCY := rectCenter(startBox)
	defenderCX, defenderCY := rectCenter(defenderBox)
	for _, terrain := range terrains {
		if terrain.Type != TerrainPassableObstacle || terrain.Shape != "rect" {
			continue
		}
		terrainBox := rectBounds{minX: terrain.X, minY: terrain.Y, maxX: terrain.X + terrain.Width, maxY: terrain.Y + terrain.Height}
		if rectsOverlap(contactBox, terrainBox) || rectsOverlap(defenderBox, terrainBox) || segmentIntersectsRect(startCX, startCY, defenderCX, defenderCY, terrainBox) {
			return true
		}
	}
	return false
}

func rectCenter(box rectBounds) (float64, float64) {
	return (box.minX + box.maxX) / 2, (box.minY + box.maxY) / 2
}

func segmentIntersectsRect(x1, y1, x2, y2 float64, box rectBounds) bool {
	if pointInsideRect(x1, y1, box) || pointInsideRect(x2, y2, box) {
		return true
	}
	return segmentsIntersect(x1, y1, x2, y2, box.minX, box.minY, box.maxX, box.minY) ||
		segmentsIntersect(x1, y1, x2, y2, box.maxX, box.minY, box.maxX, box.maxY) ||
		segmentsIntersect(x1, y1, x2, y2, box.maxX, box.maxY, box.minX, box.maxY) ||
		segmentsIntersect(x1, y1, x2, y2, box.minX, box.maxY, box.minX, box.minY)
}

func pointInsideRect(x, y float64, box rectBounds) bool {
	return x >= box.minX && x <= box.maxX && y >= box.minY && y <= box.maxY
}

func segmentsIntersect(ax, ay, bx, by, cx, cy, dx, dy float64) bool {
	o1 := segmentOrientation(ax, ay, bx, by, cx, cy)
	o2 := segmentOrientation(ax, ay, bx, by, dx, dy)
	o3 := segmentOrientation(cx, cy, dx, dy, ax, ay)
	o4 := segmentOrientation(cx, cy, dx, dy, bx, by)
	if o1 == 0 && pointOnSegment(cx, cy, ax, ay, bx, by) {
		return true
	}
	if o2 == 0 && pointOnSegment(dx, dy, ax, ay, bx, by) {
		return true
	}
	if o3 == 0 && pointOnSegment(ax, ay, cx, cy, dx, dy) {
		return true
	}
	if o4 == 0 && pointOnSegment(bx, by, cx, cy, dx, dy) {
		return true
	}
	return (o1 > 0) != (o2 > 0) && (o3 > 0) != (o4 > 0)
}

func segmentOrientation(ax, ay, bx, by, cx, cy float64) float64 {
	cross := (by-ay)*(cx-bx) - (bx-ax)*(cy-by)
	if math.Abs(cross) < 0.000001 {
		return 0
	}
	return cross
}

func pointOnSegment(px, py, ax, ay, bx, by float64) bool {
	return px >= math.Min(ax, bx)-0.000001 && px <= math.Max(ax, bx)+0.000001 &&
		py >= math.Min(ay, by)-0.000001 && py <= math.Max(ay, by)+0.000001
}

func contactedFace(defender, attacker Unit, attackerX, attackerY float64) string {
	defenderBox := unitBoundsAt(defender, defender.X, defender.Y)
	attackerBox := unitBoundsAt(attacker, attackerX, attackerY)
	dx := ((attackerBox.minX + attackerBox.maxX) / 2) - ((defenderBox.minX + defenderBox.maxX) / 2)
	dy := ((attackerBox.minY + attackerBox.maxY) / 2) - ((defenderBox.minY + defenderBox.maxY) / 2)
	localX, localY := rotatePoint(dx, dy, -defender.FacingDeg)
	if math.Abs(localY) >= math.Abs(localX) {
		if localY < 0 {
			return CombatFaceFront
		}
		return CombatFaceRear
	}
	if localX > 0 {
		return CombatFaceRight
	}
	return CombatFaceLeft
}

func snapAttackerFlush(attacker *Unit, defender Unit, defenderFace string, battlemap Battlemap, units []Unit) bool {
	officer, err := pivotAnchor(attacker, "")
	if err != nil {
		return false
	}
	faceMidX, faceMidY := defenderFaceMidpoint(defender, defenderFace)
	startX := attacker.X
	startY := attacker.Y
	startFacing := attacker.FacingDeg
	attacker.FacingDeg = attackerFacingForDefenderFace(defender.FacingDeg, defenderFace)
	frontOffset := miniCenterY(officer) - activeLocalBounds(*attacker).minY
	normalX, normalY := facingVector(attacker.FacingDeg, 1)
	officerX := faceMidX - normalX*frontOffset
	officerY := faceMidY - normalY*frontOffset
	relX, relY := rotatePoint(miniCenterX(officer), miniCenterY(officer), attacker.FacingDeg)
	for offset := 0.0; offset <= 100; offset += combatSnapStepMM {
		attacker.X = officerX - relX - normalX*offset
		attacker.Y = officerY - relY - normalY*offset
		if combatPoseValid(*attacker, defender.ID, battlemap, units) {
			return true
		}
	}
	attacker.X = startX
	attacker.Y = startY
	attacker.FacingDeg = startFacing
	return false
}

func defenderFaceMidpoint(defender Unit, face string) (float64, float64) {
	box := activeLocalBounds(defender)
	x := (box.minX + box.maxX) / 2
	y := (box.minY + box.maxY) / 2
	switch face {
	case CombatFaceFront:
		y = box.minY
	case CombatFaceRear:
		y = box.maxY
	case CombatFaceRight:
		x = box.maxX
	case CombatFaceLeft:
		x = box.minX
	}
	rotX, rotY := rotatePoint(x, y, defender.FacingDeg)
	return defender.X + rotX, defender.Y + rotY
}

func attackerFacingForDefenderFace(defenderFacing int, face string) int {
	switch face {
	case CombatFaceFront:
		return normalizeDeg(defenderFacing + 180)
	case CombatFaceRear:
		return normalizeDeg(defenderFacing)
	case CombatFaceRight:
		return normalizeDeg(defenderFacing + 270)
	case CombatFaceLeft:
		return normalizeDeg(defenderFacing + 90)
	default:
		return normalizeDeg(defenderFacing + 180)
	}
}

func combatPoseValid(unit Unit, defenderID string, battlemap Battlemap, units []Unit) bool {
	if !unitInsideBattlemap(unit, unit.X, unit.Y, battlemap) || unitOverlapsTerrain(unit, unit.X, unit.Y, battlemap.Terrains, TerrainImpassable) {
		return false
	}
	for _, other := range units {
		if other.ID == unit.ID || !other.Placed || other.Broken {
			continue
		}
		if other.ID == defenderID {
			if unitsMiniRectsOverlap(unit, unit.X, unit.Y, other) {
				return false
			}
			if !unitsMiniRectsTouch(unit, unit.X, unit.Y, other) && !unitsMiniRectsNearTouch(unit, unit.X, unit.Y, other, combatContactToleranceMM) {
				return false
			}
			continue
		}
		if unitsMiniRectsOverlap(unit, unit.X, unit.Y, other) {
			return false
		}
	}
	return true
}

func (e *Engine) resolveCombatsForUnit(g *Game, unitID string, actionIndex int) []CombatRoundResult {
	var results []CombatRoundResult
	moraleTested := moraleTestedThisRound(g)
	for _, engagement := range g.Engagements {
		if !engagement.Active {
			continue
		}
		if engagement.AttackerUnitID != unitID && engagement.DefenderUnitID != unitID {
			continue
		}
		results = append(results, e.resolveCombatRound(g, engagement, actionIndex, unitID, moraleTested))
		if g.PendingCombatChoice != nil {
			break
		}
	}
	return results
}

func (e *Engine) resolveCombatRound(g *Game, engagement CombatEngagement, actionIndex int, activeUnitID string, moraleTested map[string]bool) CombatRoundResult {
	attacker, attackerOK := findUnit(g, engagement.AttackerUnitID)
	defender, defenderOK := findUnit(g, engagement.DefenderUnitID)
	result := CombatRoundResult{EngagementID: engagement.ID}
	if !attackerOK || !defenderOK || attacker.Broken || defender.Broken {
		deactivateEngagement(g, engagement.ID)
		return result
	}
	initializeMiniHealth(attacker)
	initializeMiniHealth(defender)

	result.Attacker = e.combatSideResult(g, attacker, defender, CombatFaceFront, engagement.DefenderFace, activeUnitID, engagement.DefenderFortified)
	result.Defender = e.combatSideResult(g, defender, attacker, engagement.DefenderFace, CombatFaceFront, activeUnitID, false)

	defenderHits := applyHitsToUnit(defender, result.Attacker.Hits)
	attackerHits := applyHitsToUnit(attacker, result.Defender.Hits)
	result.Casualties = append(result.Casualties, defenderHits.Casualties...)
	result.Casualties = append(result.Casualties, attackerHits.Casualties...)
	removeUnitIfNoActiveMinis(defender)
	removeUnitIfNoActiveMinis(attacker)

	if moraleTested == nil {
		moraleTested = moraleTestedThisRound(g)
	}
	if moraleRequired(*defender, defenderHits) {
		if activeMiniCount(*defender) > 0 {
			if morale, ok := e.resolveMoraleOnce(g, defender, false, moraleTested); ok {
				result.MoraleTests = append(result.MoraleTests, morale)
			}
		}
		if defender.Broken {
			result.MoraleTests = append(result.MoraleTests, e.resolveBrokenCascade(g, defender.ID, moraleTested)...)
		}
	}
	if moraleRequired(*attacker, attackerHits) {
		if activeMiniCount(*attacker) > 0 {
			if morale, ok := e.resolveMoraleOnce(g, attacker, false, moraleTested); ok {
				result.MoraleTests = append(result.MoraleTests, morale)
			}
		}
		if attacker.Broken {
			result.MoraleTests = append(result.MoraleTests, e.resolveBrokenCascade(g, attacker.ID, moraleTested)...)
		}
	}
	result.BrokenUnits = append(result.BrokenUnits, brokenUnitsFromMorale(result.MoraleTests)...)
	if defender.Broken {
		result.BrokenUnits = appendUniqueString(result.BrokenUnits, defender.ID)
	}
	if attacker.Broken {
		result.BrokenUnits = appendUniqueString(result.BrokenUnits, attacker.ID)
	}

	if attacker.Broken || defender.Broken || activeMiniCount(*attacker) == 0 || activeMiniCount(*defender) == 0 {
		deactivateEngagement(g, engagement.ID)
		return result
	}
	if result.Attacker.Hits > result.Defender.Hits {
		result.WinnerUnitID = attacker.ID
		result.PendingChoice = createPendingCombatChoice(engagement, *attacker, *defender, actionIndex)
		g.PendingCombatChoice = result.PendingChoice
	} else if result.Defender.Hits > result.Attacker.Hits {
		result.WinnerUnitID = defender.ID
		result.PendingChoice = createPendingCombatChoice(engagement, *defender, *attacker, actionIndex)
		g.PendingCombatChoice = result.PendingChoice
	} else {
		result.TiePushback = applyTiePushback(g, *attacker, *defender)
		deactivateEngagement(g, engagement.ID)
	}
	return result
}

func (e *Engine) combatSideResult(g *Game, attacker, defender *Unit, ownContactFace, defenderFace, activeUnitID string, defenderFortified bool) CombatSideResult {
	dice := combatDiceCount(*attacker, ownContactFace)
	target, modifiers := combatTargetNumber(g, *attacker, *defender, ownContactFace, defenderFace, activeUnitID, defenderFortified)
	rolls := make([]int, 0, dice)
	hits := 0
	for i := 0; i < dice; i++ {
		roll := rollD10(g)
		rolls = append(rolls, roll)
		hits += hitsForRoll(roll, target)
	}
	return CombatSideResult{
		UnitID:       attacker.ID,
		PlayerID:     attacker.PlayerID,
		ContactFace:  ownContactFace,
		DiceCount:    dice,
		TargetNumber: target,
		Modifiers:    modifiers,
		Rolls:        rolls,
		Hits:         hits,
	}
}

func combatDiceCount(unit Unit, ownContactFace string) int {
	multiplier := activeFullRanks(unit)
	if ownContactFace == CombatFaceFront {
		multiplier = activeFrontRankCount(unit)
	}
	dice := unit.Stats.CD * multiplier
	if dice < 1 {
		return 1
	}
	return dice
}

func combatTargetNumber(g *Game, attacker, defender Unit, ownContactFace, defenderFace, activeUnitID string, defenderFortified bool) (int, []CombatModifier) {
	target := defender.Stats.D - attacker.Stats.A
	var modifiers []CombatModifier
	add := func(label string, value int) {
		if value == 0 {
			return
		}
		modifiers = append(modifiers, CombatModifier{Label: label, Value: value})
		target += value
	}
	if ownContactFace == CombatFaceFront {
		add("ranks", -(activeFullRanks(attacker) - 1))
	}
	if defenderFace != CombatFaceFront {
		add("attacking flank or rear", -1)
	}
	if defenderFace == CombatFaceRear {
		add("defender rear face", 1)
	}
	if attacker.Disordered {
		add("attacker disordered", 1)
	}
	add("lower elevation", lowerElevationModifier(attacker, defender))
	add("defender behind fortification", fortificationModifier(defenderFortified))
	return target, modifiers
}

func lowerElevationModifier(attacker, defender Unit) int {
	_ = attacker
	_ = defender
	return 0
}

func fortificationModifier(defenderFortified bool) int {
	if defenderFortified {
		return 1
	}
	return 0
}

func hitsForRoll(roll, target int) int {
	if roll < target {
		return 0
	}
	if roll-target >= 10 {
		return 3
	}
	if roll-target >= 5 {
		return 2
	}
	return 1
}

type hitApplication struct {
	Casualties []CasualtyResult
	Removed    int
	Damage     int
}

func applyHitsToUnit(unit *Unit, hits int) hitApplication {
	initializeMiniHealth(unit)
	var result hitApplication
	for i := 0; i < hits; i++ {
		targetIndex := nextHitMiniIndex(*unit)
		if targetIndex < 0 {
			break
		}
		mini := &unit.Minis[targetIndex]
		healthBefore := mini.HealthRemaining
		mini.HealthRemaining--
		healthAfter := mini.HealthRemaining
		if healthAfter < 0 {
			healthAfter = 0
		}
		result.Damage++
		casualty := CasualtyResult{
			UnitID:       unit.ID,
			MiniKey:      mini.Key,
			Damage:       1,
			HealthBefore: healthBefore,
			HealthAfter:  healthAfter,
			WasOfficer:   mini.IsOfficer,
		}
		if mini.HealthRemaining <= 0 {
			mini.HealthRemaining = 0
			mini.Removed = true
			result.Removed++
			casualty.Removed = true
		}
		result.Casualties = append(result.Casualties, casualty)
	}
	return result
}

func nextHitMiniIndex(unit Unit) int {
	best := -1
	for i, mini := range unit.Minis {
		if mini.Removed || mini.IsOfficer || miniHealth(unit, mini) <= 0 {
			continue
		}
		if best < 0 || mini.Index > unit.Minis[best].Index {
			best = i
		}
	}
	if best >= 0 {
		return best
	}
	for i, mini := range unit.Minis {
		if mini.Removed || !mini.IsOfficer || miniHealth(unit, mini) <= 0 {
			continue
		}
		if best < 0 || mini.Index > unit.Minis[best].Index {
			best = i
		}
	}
	return best
}

func moraleRequired(unit Unit, hits hitApplication) bool {
	if len(unit.Minis) == 1 {
		return hits.Damage > 0
	}
	return hits.Removed > 0
}

func removeUnitIfNoActiveMinis(unit *Unit) {
	if activeMiniCount(*unit) > 0 {
		return
	}
	unit.Broken = true
	unit.Placed = false
}

func (e *Engine) resolveMoraleTest(g *Game, unit *Unit, cascade bool) MoraleTestResult {
	target, modifiers := moraleTargetNumber(*unit, cascade, false)
	rolls := []int{rollD10(g), rollD10(g)}
	passed := rolls[0] >= target || rolls[1] >= target
	outcome := "passed"
	wasDisordered := unit.Disordered
	if !passed {
		if wasDisordered {
			unit.Broken = true
			unit.Placed = false
			deactivateEngagementsForUnit(g, unit.ID)
			outcome = UnitStatusBroken
		} else {
			unit.Disordered = true
			outcome = UnitStatusDisordered
		}
	}
	return MoraleTestResult{
		UnitID:       unit.ID,
		Rolls:        rolls,
		TargetNumber: target,
		Modifiers:    modifiers,
		Passed:       passed,
		Cascade:      cascade,
		Outcome:      outcome,
	}
}

func (e *Engine) resolveMoraleOnce(g *Game, unit *Unit, cascade bool, tested map[string]bool) (MoraleTestResult, bool) {
	if tested[unit.ID] {
		return MoraleTestResult{}, false
	}
	morale := e.resolveMoraleTest(g, unit, cascade)
	tested[unit.ID] = true
	return morale, true
}

func moraleTargetNumber(unit Unit, cascade, shooting bool) (int, []CombatModifier) {
	target := unit.Stats.A
	var modifiers []CombatModifier
	if len(unit.Minis) == 1 || cascade {
		return target, modifiers
	}
	fullRanks := activeFullRanks(unit)
	add := func(label string, value int) {
		if value == 0 {
			return
		}
		modifiers = append(modifiers, CombatModifier{Label: label, Value: value})
		target += value
	}
	add("casualties", -casualtyCount(unit))
	if unit.Disordered {
		add("disordered", -1)
	}
	if fullRanks < 1 {
		add("less than one full rank", -1)
	}
	if unit.Base.WidthMM == 25 && unit.Base.DepthMM == 25 && fullRanks >= 2 {
		add("25x25 two full ranks", 1)
	}
	if ((unit.Base.WidthMM == 25 && unit.Base.DepthMM == 50) || (unit.Base.WidthMM == 50 && unit.Base.DepthMM == 50)) && fullRanks >= 1 {
		add("larger base full rank", 1)
	}
	if shooting {
		add("shooting attack", 1)
	}
	return target, modifiers
}

func (e *Engine) resolveBrokenCascade(g *Game, brokenUnitID string, tested map[string]bool) []MoraleTestResult {
	var results []MoraleTestResult
	queued := map[string]bool{brokenUnitID: true}
	queue := []string{brokenUnitID}
	for len(queue) > 0 {
		sourceID := queue[0]
		queue = queue[1:]
		source, ok := findUnit(g, sourceID)
		if !ok {
			continue
		}
		sx, sy := cascadeSourceCenter(*source)
		for i := range g.Units {
			unit := &g.Units[i]
			if queued[unit.ID] || unit.Broken || !unit.Placed || unit.PlayerID != source.PlayerID {
				continue
			}
			ux, uy := unitCenter(*unit)
			if math.Hypot(ux-sx, uy-sy) > BrokenMoraleRangeMM {
				continue
			}
			queued[unit.ID] = true
			morale, ok := e.resolveMoraleOnce(g, unit, true, tested)
			if !ok {
				continue
			}
			results = append(results, morale)
			if unit.Broken {
				queue = append(queue, unit.ID)
			}
		}
	}
	return results
}

func moraleTestedThisRound(g *Game) map[string]bool {
	tested := map[string]bool{}
	for _, rec := range g.ActionHistory {
		if rec.Round == g.Round {
			markMoraleTestsInResult(tested, rec.Result)
		}
	}
	return tested
}

func markMoraleTestsInResult(tested map[string]bool, result any) {
	value, ok := result.(map[string]any)
	if !ok {
		return
	}
	markMoraleTestsInCombat(tested, value["combatRound"])
	markMoraleTestsInCombat(tested, value["movement"])
	markMoraleTestsInCombat(tested, value["shooting"])
	if rounds, ok := value["combatRounds"].([]CombatRoundResult); ok {
		for _, round := range rounds {
			markMoraleTestsInCombat(tested, round)
		}
		return
	}
	if rounds, ok := value["combatRounds"].([]any); ok {
		for _, round := range rounds {
			markMoraleTestsInCombat(tested, round)
		}
	}
}

func markMoraleTestsInCombat(tested map[string]bool, combat any) {
	switch value := combat.(type) {
	case CombatRoundResult:
		for _, morale := range value.MoraleTests {
			tested[morale.UnitID] = true
		}
	case ShootResult:
		for _, morale := range value.MoraleTests {
			tested[morale.UnitID] = true
		}
	case *CombatRoundResult:
		if value != nil {
			markMoraleTestsInCombat(tested, *value)
		}
	case *ShootResult:
		if value != nil {
			markMoraleTestsInCombat(tested, *value)
		}
	case MoveResult:
		if value.Combat != nil {
			markMoraleTestsInCombat(tested, value.Combat)
		}
	case map[string]any:
		if nested, ok := value["combat"]; ok {
			markMoraleTestsInCombat(tested, nested)
		}
		tests, ok := value["moraleTests"].([]any)
		if !ok {
			return
		}
		for _, test := range tests {
			morale, ok := test.(map[string]any)
			if !ok {
				continue
			}
			unitID, ok := morale["unitId"].(string)
			if ok && unitID != "" {
				tested[unitID] = true
			}
		}
	}
}

func brokenUnitsFromMorale(results []MoraleTestResult) []string {
	seen := map[string]bool{}
	var out []string
	for _, result := range results {
		if result.Outcome != UnitStatusBroken || seen[result.UnitID] {
			continue
		}
		seen[result.UnitID] = true
		out = append(out, result.UnitID)
	}
	return out
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func createPendingCombatChoice(engagement CombatEngagement, winner, loser Unit, actionIndex int) *PendingCombatChoice {
	return &PendingCombatChoice{
		EngagementID:      engagement.ID,
		WinningPlayerID:   winner.PlayerID,
		WinningUnitID:     winner.ID,
		WinningIsAttacker: winner.ID == engagement.AttackerUnitID,
		LosingUnitID:      loser.ID,
		Choices:           []string{CombatChoicePushback150, CombatChoiceWithdraw25, CombatChoiceDecline},
		AxisDX:            engagement.AxisDX,
		AxisDY:            engagement.AxisDY,
		SourceActionIndex: actionIndex,
	}
}

func applyTiePushback(g *Game, attacker, defender Unit) []CombatChoiceResult {
	attackerCX, attackerCY := unitCenter(attacker)
	defenderCX, defenderCY := unitCenter(defender)
	dx := attackerCX - defenderCX
	dy := attackerCY - defenderCY
	length := math.Hypot(dx, dy)
	if length == 0 {
		forwardX, forwardY := facingVector(attacker.FacingDeg, 1)
		dx = -forwardX
		dy = -forwardY
		length = math.Hypot(dx, dy)
	}
	dx /= length
	dy /= length
	attackerResult := moveCombatChoiceUnit(g, attacker.ID, dx, dy, 25, defender.ID)
	attackerResult.Choice = "tie_pushback"
	defenderResult := moveCombatChoiceUnit(g, defender.ID, -dx, -dy, 25, attacker.ID)
	defenderResult.Choice = "tie_pushback"
	return []CombatChoiceResult{attackerResult, defenderResult}
}

func deactivateEngagement(g *Game, engagementID string) {
	for i := range g.Engagements {
		if g.Engagements[i].ID == engagementID {
			g.Engagements[i].Active = false
		}
	}
}

func deactivateEngagementsForUnit(g *Game, unitID string) {
	for i := range g.Engagements {
		if g.Engagements[i].AttackerUnitID == unitID || g.Engagements[i].DefenderUnitID == unitID {
			g.Engagements[i].Active = false
		}
	}
}

func combatMessages(result CombatRoundResult) []string {
	messages := []string{
		fmt.Sprintf("Combat %s: %s rolled %v vs TN %d for %d hit(s); %s rolled %v vs TN %d for %d hit(s).",
			result.EngagementID,
			result.Attacker.UnitID, result.Attacker.Rolls, result.Attacker.TargetNumber, result.Attacker.Hits,
			result.Defender.UnitID, result.Defender.Rolls, result.Defender.TargetNumber, result.Defender.Hits),
	}
	for _, casualty := range result.Casualties {
		if casualty.Removed {
			messages = append(messages, fmt.Sprintf("%s removed from %s.", casualty.MiniKey, casualty.UnitID))
		}
	}
	for _, morale := range result.MoraleTests {
		messages = append(messages, fmt.Sprintf("%s morale rolled %v against %d: %s.", morale.UnitID, morale.Rolls, morale.TargetNumber, morale.Outcome))
	}
	if result.PendingChoice != nil {
		messages = append(messages, fmt.Sprintf("%s won combat; choose pushback, withdraw, or decline.", result.WinnerUnitID))
	}
	if len(result.TiePushback) > 0 {
		for _, pushback := range result.TiePushback {
			messages = append(messages, fmt.Sprintf("Tied combat pushed %s %.0fmm.", pushback.MovingUnitID, pushback.MovedDistanceMM))
		}
	}
	return messages
}

func pivotAnchor(unit *Unit, key string) (Mini, error) {
	if key != "" {
		for _, mini := range unit.Minis {
			if mini.Key == key {
				return mini, nil
			}
		}
		return Mini{}, errors.New("pivot anchor mini key is not in the unit")
	}
	for _, mini := range unit.Minis {
		if mini.IsOfficer {
			return mini, nil
		}
	}
	return Mini{}, errors.New("unit has no officer to pivot around")
}

func applyPivot(unit *Unit, anchor Mini, facingDeg int, terrains []TerrainZone, units []Unit) {
	anchorX, anchorY := miniWorldCenter(*unit, anchor, unit.FacingDeg)
	startFacing := unit.FacingDeg
	targetFacing := normalizeDeg(facingDeg)
	delta := shortestSignedDelta(startFacing, targetFacing)
	if delta == 0 {
		return
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	for remaining := int(math.Abs(float64(delta))); remaining > 0; remaining-- {
		nextFacing := normalizeDeg(unit.FacingDeg + step)
		nextX, nextY := pivotOriginForAnchor(anchor, anchorX, anchorY, nextFacing)
		candidate := *unit
		candidate.X = nextX
		candidate.Y = nextY
		candidate.FacingDeg = nextFacing
		if unitOverlapsTerrain(candidate, nextX, nextY, terrains, TerrainImpassable) || unitOverlapsAnyUnit(candidate, nextX, nextY, units) {
			return
		}
		unit.X = nextX
		unit.Y = nextY
		unit.FacingDeg = nextFacing
	}
}

func validateUnitPosition(unit Unit, battlemap Battlemap, units []Unit) error {
	if !unitInsideBattlemap(unit, unit.X, unit.Y, battlemap) {
		return errors.New("action must keep the whole unit in the arena")
	}
	if unitOverlapsTerrain(unit, unit.X, unit.Y, battlemap.Terrains, TerrainImpassable) {
		return errors.New("action overlaps impassable terrain")
	}
	if unitOverlapsAnyUnit(unit, unit.X, unit.Y, units) {
		return errors.New("action overlaps another unit")
	}
	return nil
}

func pivotOriginForAnchor(anchor Mini, anchorX, anchorY float64, facingDeg int) (float64, float64) {
	relX, relY := rotatePoint(miniCenterX(anchor), miniCenterY(anchor), facingDeg)
	return anchorX - relX, anchorY - relY
}

func shortestSignedDelta(from, to int) int {
	delta := normalizeDeg(to) - normalizeDeg(from)
	if delta > 180 {
		delta -= 360
	}
	if delta < -180 {
		delta += 360
	}
	return delta
}

func (g *Game) appendRecord(kind string, player int, unitID string, request, result any, messages []string) ActionRecord {
	rec := ActionRecord{
		Index:     len(g.ActionHistory),
		Round:     g.Round,
		Type:      kind,
		PlayerID:  player,
		UnitID:    unitID,
		Request:   request,
		Result:    result,
		Messages:  messages,
		CreatedAt: time.Now().UTC(),
	}
	g.ActionHistory = append(g.ActionHistory, rec)
	return rec
}

func Snapshot(g *Game) (string, error) {
	clone := *g
	clone.Snapshots = nil
	b, err := json.Marshal(clone)
	return string(b), err
}

func Restore(snapshot string) (*Game, error) {
	var g Game
	if err := json.Unmarshal([]byte(snapshot), &g); err != nil {
		return nil, err
	}
	NormalizeGame(&g)
	return &g, nil
}

func NormalizeGame(g *Game) {
	if g == nil {
		return
	}
	NormalizeBattlemap(&g.Battlemap)
	if g.Engagements == nil {
		g.Engagements = []CombatEngagement{}
	}
	for i := range g.Units {
		initializeMiniHealth(&g.Units[i])
	}
}

func LegalActions(g *Game) []string {
	if g.Phase == "complete" {
		return nil
	}
	if g.PendingCombatChoice != nil {
		return []string{ActionCombatPushback}
	}
	if g.Phase == "setup" {
		return []string{ActionPlace}
	}
	if g.CurrentActivation == nil {
		return []string{ActionActivate}
	}
	actions := []string{ActionMove, ActionPivot, ActionAboutFace}
	if len(legalShootTargets(g)) > 0 {
		actions = append(actions, ActionShoot)
	}
	actions = append(actions, ActionSkip)
	return actions
}

func placementUnitID(g *Game) (string, bool) {
	player := g.ActivePlayer
	for checked := 0; checked < 2; checked++ {
		for _, unit := range g.Units {
			if unit.PlayerID == player && !unit.Placed && activeMiniCount(unit) > 0 {
				return unit.ID, true
			}
		}
		player = otherPlayer(player)
	}
	return "", false
}

func nextPlacementPlayer(g *Game) int {
	other := otherPlayer(g.ActivePlayer)
	if playerHasUnplacedUnit(g, other) {
		return other
	}
	return g.ActivePlayer
}

func playerHasUnplacedUnit(g *Game, playerID int) bool {
	for _, unit := range g.Units {
		if unit.PlayerID == playerID && !unit.Placed && activeMiniCount(unit) > 0 {
			return true
		}
	}
	return false
}

func allUnitsPlaced(g *Game) bool {
	for _, unit := range g.Units {
		if !unit.Placed && activeMiniCount(unit) > 0 {
			return false
		}
	}
	return true
}

func findUnit(g *Game, id string) (*Unit, bool) {
	for i := range g.Units {
		if g.Units[i].ID == id {
			return &g.Units[i], true
		}
	}
	return nil, false
}

func hasMini(unit *Unit, key string) bool {
	for _, mini := range unit.Minis {
		if mini.Key == key {
			return true
		}
	}
	return false
}

type rectBounds struct {
	minX float64
	minY float64
	maxX float64
	maxY float64
}

func unitOverlapsTerrain(unit Unit, x, y float64, terrains []TerrainZone, terrainType string) bool {
	unitBox := unitBoundsAt(unit, x, y)
	for _, terrain := range terrains {
		if terrain.Type != terrainType || terrain.Shape != "rect" {
			continue
		}
		terrainBox := rectBounds{minX: terrain.X, minY: terrain.Y, maxX: terrain.X + terrain.Width, maxY: terrain.Y + terrain.Height}
		if rectsOverlap(unitBox, terrainBox) {
			return true
		}
	}
	return false
}

func unitBoundsAt(unit Unit, x, y float64) rectBounds {
	return unitBoundsAtFiltered(unit, x, y, false)
}

func unitBoundsAtFiltered(unit Unit, x, y float64, includeRemoved bool) rectBounds {
	box := rectBounds{minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
	for _, mini := range unit.Minis {
		if mini.Removed && !includeRemoved {
			continue
		}
		corners := [][2]float64{
			{mini.RelX, mini.RelY},
			{mini.RelX + float64(mini.WidthMM), mini.RelY},
			{mini.RelX + float64(mini.WidthMM), mini.RelY + float64(mini.DepthMM)},
			{mini.RelX, mini.RelY + float64(mini.DepthMM)},
		}
		for _, corner := range corners {
			rotatedX, rotatedY := rotatePoint(corner[0], corner[1], unit.FacingDeg)
			worldX := x + rotatedX
			worldY := y + rotatedY
			box.minX = math.Min(box.minX, worldX)
			box.minY = math.Min(box.minY, worldY)
			box.maxX = math.Max(box.maxX, worldX)
			box.maxY = math.Max(box.maxY, worldY)
		}
	}
	if math.IsInf(box.minX, 1) {
		return rectBounds{minX: x, minY: y, maxX: x, maxY: y}
	}
	return box
}

func unitAllMiniBoundsAt(unit Unit, x, y float64) rectBounds {
	return unitBoundsAtFiltered(unit, x, y, true)
}

func rectsOverlap(a, b rectBounds) bool {
	return a.minX < b.maxX && a.maxX > b.minX && a.minY < b.maxY && a.maxY > b.minY
}

func unitOverlapsEnemyUnit(unit Unit, x, y float64, units []Unit) bool {
	return unitOverlapsOtherUnit(unit, x, y, units, false)
}

func unitOverlapsFriendlyUnit(unit Unit, x, y float64, units []Unit) bool {
	return unitOverlapsOtherUnit(unit, x, y, units, true)
}

func unitOverlapsAnyUnit(unit Unit, x, y float64, units []Unit) bool {
	for _, other := range units {
		if other.ID == unit.ID || !other.Placed || other.Broken {
			continue
		}
		if unitsMiniRectsOverlap(unit, x, y, other) {
			return true
		}
	}
	return false
}

func unitOverlapsOtherUnit(unit Unit, x, y float64, units []Unit, friendly bool) bool {
	for _, other := range units {
		if other.ID == unit.ID || !other.Placed || other.Broken {
			continue
		}
		if (other.PlayerID == unit.PlayerID) != friendly {
			continue
		}
		if unitsMiniRectsOverlap(unit, x, y, other) {
			return true
		}
	}
	return false
}

func unitsMiniRectsOverlap(unit Unit, x, y float64, other Unit) bool {
	for _, mini := range unit.Minis {
		if mini.Removed {
			continue
		}
		poly := miniWorldPolygon(unit, mini, x, y)
		for _, otherMini := range other.Minis {
			if otherMini.Removed {
				continue
			}
			if polygonsOverlap(poly, miniWorldPolygon(other, otherMini, other.X, other.Y)) {
				return true
			}
		}
	}
	return false
}

const (
	combatSnapStepMM         = 0.05
	combatContactToleranceMM = 0.1
)

func unitsMiniRectsTouch(unit Unit, x, y float64, other Unit) bool {
	for _, mini := range unit.Minis {
		if mini.Removed {
			continue
		}
		box := polygonBounds(miniWorldPolygon(unit, mini, x, y))
		for _, otherMini := range other.Minis {
			if otherMini.Removed {
				continue
			}
			if rectsTouch(box, polygonBounds(miniWorldPolygon(other, otherMini, other.X, other.Y))) {
				return true
			}
		}
	}
	return false
}

func unitsMiniRectsNearTouch(unit Unit, x, y float64, other Unit, tolerance float64) bool {
	for _, mini := range unit.Minis {
		if mini.Removed {
			continue
		}
		poly := miniWorldPolygon(unit, mini, x, y)
		for _, otherMini := range other.Minis {
			if otherMini.Removed {
				continue
			}
			if polygonDistance(poly, miniWorldPolygon(other, otherMini, other.X, other.Y)) <= tolerance {
				return true
			}
		}
	}
	return false
}

func polygonBounds(poly [4][2]float64) rectBounds {
	box := rectBounds{minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
	for _, point := range poly {
		box.minX = math.Min(box.minX, point[0])
		box.minY = math.Min(box.minY, point[1])
		box.maxX = math.Max(box.maxX, point[0])
		box.maxY = math.Max(box.maxY, point[1])
	}
	return box
}

func rectsTouch(a, b rectBounds) bool {
	const eps = 0.000001
	xOverlap := a.minX < b.maxX-eps && a.maxX > b.minX+eps
	yOverlap := a.minY < b.maxY-eps && a.maxY > b.minY+eps
	xTouch := math.Abs(a.maxX-b.minX) <= eps || math.Abs(b.maxX-a.minX) <= eps
	yTouch := math.Abs(a.maxY-b.minY) <= eps || math.Abs(b.maxY-a.minY) <= eps
	return (xTouch && yOverlap) || (yTouch && xOverlap)
}

func miniWorldPolygon(unit Unit, mini Mini, unitX, unitY float64) [4][2]float64 {
	corners := [4][2]float64{
		{mini.RelX, mini.RelY},
		{mini.RelX + float64(mini.WidthMM), mini.RelY},
		{mini.RelX + float64(mini.WidthMM), mini.RelY + float64(mini.DepthMM)},
		{mini.RelX, mini.RelY + float64(mini.DepthMM)},
	}
	for i, corner := range corners {
		x, y := rotatePoint(corner[0], corner[1], unit.FacingDeg)
		corners[i] = [2]float64{unitX + x, unitY + y}
	}
	return corners
}

func polygonsOverlap(a, b [4][2]float64) bool {
	for _, poly := range [][4][2]float64{a, b} {
		for i := 0; i < len(poly); i++ {
			next := (i + 1) % len(poly)
			edgeX := poly[next][0] - poly[i][0]
			edgeY := poly[next][1] - poly[i][1]
			axisX := -edgeY
			axisY := edgeX
			minA, maxA := projectPolygon(a, axisX, axisY)
			minB, maxB := projectPolygon(b, axisX, axisY)
			if maxA <= minB || maxB <= minA {
				return false
			}
		}
	}
	return true
}

func polygonDistance(a, b [4][2]float64) float64 {
	if polygonsOverlap(a, b) {
		return 0
	}
	distance := math.Inf(1)
	for i := 0; i < len(a); i++ {
		aNext := (i + 1) % len(a)
		for j := 0; j < len(b); j++ {
			bNext := (j + 1) % len(b)
			distance = math.Min(distance, segmentDistance(
				a[i][0], a[i][1],
				a[aNext][0], a[aNext][1],
				b[j][0], b[j][1],
				b[bNext][0], b[bNext][1],
			))
		}
	}
	return distance
}

func segmentDistance(ax, ay, bx, by, cx, cy, dx, dy float64) float64 {
	if segmentsIntersect(ax, ay, bx, by, cx, cy, dx, dy) {
		return 0
	}
	return minFloat(
		minFloat(pointSegmentDistance(ax, ay, cx, cy, dx, dy), pointSegmentDistance(bx, by, cx, cy, dx, dy)),
		minFloat(pointSegmentDistance(cx, cy, ax, ay, bx, by), pointSegmentDistance(dx, dy, ax, ay, bx, by)),
	)
}

func pointSegmentDistance(px, py, ax, ay, bx, by float64) float64 {
	dx := bx - ax
	dy := by - ay
	lengthSquared := dx*dx + dy*dy
	if lengthSquared == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / lengthSquared
	t = math.Max(0, math.Min(1, t))
	closestX := ax + t*dx
	closestY := ay + t*dy
	return math.Hypot(px-closestX, py-closestY)
}

func projectPolygon(poly [4][2]float64, axisX, axisY float64) (float64, float64) {
	minProjection := poly[0][0]*axisX + poly[0][1]*axisY
	maxProjection := minProjection
	for i := 1; i < len(poly); i++ {
		projection := poly[i][0]*axisX + poly[i][1]*axisY
		minProjection = math.Min(minProjection, projection)
		maxProjection = math.Max(maxProjection, projection)
	}
	return minProjection, maxProjection
}

func revertToBeforeFriendlyOverlap(unit *Unit, req ActionRequest, terrains []TerrainZone, units []Unit, startX, startY, maxMoved float64) float64 {
	sign := 1.0
	if req.Direction == "backward" {
		sign = -1
	}
	rad := float64(unit.FacingDeg) * math.Pi / 180
	dx := math.Sin(rad) * sign
	dy := -math.Cos(rad) * sign
	lastClearX := startX
	lastClearY := startY
	lastClearMoved := 0.0
	inFriendly := false
	for moved := 0.0; moved < maxMoved; {
		step := minFloat(1, maxMoved-moved)
		nextMoved := moved + step
		nextX := startX + dx*nextMoved
		nextY := startY + dy*nextMoved
		if unitOverlapsTerrain(*unit, nextX, nextY, terrains, TerrainImpassable) || unitOverlapsEnemyUnit(*unit, nextX, nextY, units) {
			break
		}
		if unitOverlapsFriendlyUnit(*unit, nextX, nextY, units) {
			inFriendly = true
		} else {
			inFriendly = false
			lastClearX = nextX
			lastClearY = nextY
			lastClearMoved = nextMoved
		}
		moved = nextMoved
	}
	if inFriendly {
		unit.X = lastClearX
		unit.Y = lastClearY
		return lastClearMoved
	}
	return maxMoved
}

func unitInsideBattlemap(unit Unit, x, y float64, battlemap Battlemap) bool {
	NormalizeBattlemap(&battlemap)
	box := unitBoundsAt(unit, x, y)
	return box.minX >= 0 && box.minY >= 0 && box.maxX <= battlemap.WidthMM && box.maxY <= battlemap.HeightMM
}

func facingTowardBattlemapCenter(battlemap Battlemap, x, y float64) int {
	NormalizeBattlemap(&battlemap)
	dx := battlemap.WidthMM/2 - x
	dy := battlemap.HeightMM/2 - y
	deg := math.Atan2(dx, -dy) * 180 / math.Pi
	return normalizeDeg(int(math.Round(deg/45) * 45))
}

func miniWorldCenter(unit Unit, mini Mini, facingDeg int) (float64, float64) {
	x, y := rotatePoint(miniCenterX(mini), miniCenterY(mini), facingDeg)
	return unit.X + x, unit.Y + y
}

func rotatePoint(x, y float64, deg int) (float64, float64) {
	rad := float64(normalizeDeg(deg)) * math.Pi / 180
	return x*math.Cos(rad) - y*math.Sin(rad), x*math.Sin(rad) + y*math.Cos(rad)
}

func miniCenterX(mini Mini) float64 {
	return mini.RelX + float64(mini.WidthMM)/2
}

func miniCenterY(mini Mini) float64 {
	return mini.RelY + float64(mini.DepthMM)/2
}

func applyAboutFace(unit *Unit) error {
	count := len(unit.Minis)
	officerIndex := -1
	for i := range unit.Minis {
		if unit.Minis[i].IsOfficer {
			officerIndex = i
			break
		}
	}
	if officerIndex < 0 {
		return errors.New("unit has no officer for about face")
	}

	perRank := unit.Base.PerRank
	fullRanks := count / perRank
	if fullRanks == 0 {
		fullRanks = 1
	}
	lastFullRank := fullRanks - 1
	officerFile := unit.Minis[officerIndex].File
	swapIndex := officerIndex
	for i := range unit.Minis {
		if unit.Minis[i].Rank == lastFullRank && unit.Minis[i].File == officerFile {
			swapIndex = i
			break
		}
	}
	swapMiniPositions(&unit.Minis[officerIndex], &unit.Minis[swapIndex])
	fixedX, fixedY := miniWorldCenter(*unit, unit.Minis[officerIndex], unit.FacingDeg)

	for i := range unit.Minis {
		oldRank := unit.Minis[i].Rank
		if oldRank < fullRanks {
			unit.Minis[i].Rank = fullRanks - 1 - oldRank
		} else {
			unit.Minis[i].Rank = fullRanks + (oldRank - fullRanks)
		}
		setMiniPosition(unit, i, unit.Minis[i].Rank, unit.Minis[i].File)
	}

	nextFacing := normalizeDeg(unit.FacingDeg + 180)
	relX, relY := rotatePoint(miniCenterX(unit.Minis[officerIndex]), miniCenterY(unit.Minis[officerIndex]), nextFacing)
	unit.X = fixedX - relX
	unit.Y = fixedY - relY
	unit.FacingDeg = nextFacing
	return nil
}

func swapMiniPositions(a, b *Mini) {
	a.Rank, b.Rank = b.Rank, a.Rank
	a.File, b.File = b.File, a.File
	a.RelX, b.RelX = b.RelX, a.RelX
	a.RelY, b.RelY = b.RelY, a.RelY
}

func setMiniPosition(unit *Unit, index, rank, file int) {
	unit.Minis[index].Rank = rank
	unit.Minis[index].File = file
	unit.Minis[index].RelX = float64(file * unit.Base.WidthMM)
	unit.Minis[index].RelY = float64(rank * unit.Base.DepthMM)
}

func miniByKey(unit *Unit, key string) (Mini, bool) {
	for _, mini := range unit.Minis {
		if mini.Key == key {
			return mini, true
		}
	}
	return Mini{}, false
}

func unitActivatedThisRound(g *Game, unitID string) bool {
	for i := len(g.ActionHistory) - 1; i >= 0; i-- {
		rec := g.ActionHistory[i]
		if rec.Round != g.Round {
			continue
		}
		if rec.Type == ActionActivate && rec.UnitID == unitID {
			return true
		}
	}
	return false
}

func advanceTurn(g *Game) {
	if g.Phase == "complete" {
		return
	}
	if allUnitsActivated(g) {
		g.Round++
		g.ActivePlayer = g.FirstPlayer
		return
	}
	other := otherPlayer(g.ActivePlayer)
	if playerHasUnactivatedUnit(g, other) {
		g.ActivePlayer = other
		return
	}
	if playerHasUnactivatedUnit(g, g.ActivePlayer) {
		return
	}
	g.ActivePlayer = other
}

func otherPlayer(playerID int) int {
	if playerID == 1 {
		return 2
	}
	return 1
}

func playerHasUnactivatedUnit(g *Game, playerID int) bool {
	for _, unit := range g.Units {
		if unit.PlayerID == playerID && !unit.Broken && unit.Placed && activeMiniCount(unit) > 0 && !unitActivatedThisRound(g, unit.ID) {
			return true
		}
	}
	return false
}

func allUnitsActivated(g *Game) bool {
	seen := map[string]bool{}
	for _, rec := range g.ActionHistory {
		if rec.Round == g.Round && rec.Type == ActionActivate {
			seen[rec.UnitID] = true
		}
	}
	for _, unit := range g.Units {
		if unit.Broken || !unit.Placed || activeMiniCount(unit) == 0 {
			continue
		}
		if !seen[unit.ID] {
			return false
		}
	}
	return true
}

func completeGameIfWon(g *Game) string {
	if g.Phase == "setup" || g.Phase == "complete" || g.WinnerPlayerID != 0 {
		return ""
	}
	players := activePlayersRemaining(g)
	if len(players) == 0 {
		completeGame(g, 0)
		return "No players have active units remaining; game ends in a draw."
	}
	if len(players) != 1 {
		return ""
	}
	for playerID := range players {
		completeGame(g, playerID)
		return fmt.Sprintf("Player %d wins.", playerID)
	}
	return ""
}

func completeGame(g *Game, winnerPlayerID int) {
	g.WinnerPlayerID = winnerPlayerID
	g.Phase = "complete"
	g.CurrentActivation = nil
	g.PendingCombatChoice = nil
	for i := range g.Engagements {
		g.Engagements[i].Active = false
	}
}

func activePlayersRemaining(g *Game) map[int]bool {
	players := map[int]bool{}
	for _, unit := range g.Units {
		if unit.Broken || !unit.Placed || activeMiniCount(unit) == 0 {
			continue
		}
		players[unit.PlayerID] = true
	}
	return players
}

func rollD10(g *Game) int {
	g.RandomRollIndex++
	x := uint64(g.RandomSeed) + uint64(g.RandomRollIndex)*0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	x ^= x >> 31
	return int(x%10) + 1
}

func miniMaxHealth(unit Unit) int {
	if unit.Stats.H > 0 {
		return unit.Stats.H
	}
	if unit.MaxHealth > 0 {
		return unit.MaxHealth
	}
	return DefaultMiniHealth
}

func miniStartingHealth(unit Unit) int {
	maxHealth := miniMaxHealth(unit)
	if unit.CurrentHealthSet && unit.CurrentHealth <= 0 {
		return 0
	}
	if unit.CurrentHealth > 0 {
		return min(unit.CurrentHealth, maxHealth)
	}
	return maxHealth
}

func initializeMiniHealth(unit *Unit) {
	startingHealth := miniStartingHealth(*unit)
	for i := range unit.Minis {
		if unit.Minis[i].Removed {
			continue
		}
		if unit.Minis[i].HealthRemaining <= 0 {
			unit.Minis[i].HealthRemaining = startingHealth
		}
	}
}

func miniHealth(unit Unit, mini Mini) int {
	if mini.HealthRemaining > 0 {
		return mini.HealthRemaining
	}
	return miniMaxHealth(unit)
}

func activeMiniCount(unit Unit) int {
	count := 0
	for _, mini := range unit.Minis {
		if !mini.Removed && miniHealth(unit, mini) > 0 {
			count++
		}
	}
	return count
}

func casualtyCount(unit Unit) int {
	count := 0
	for _, mini := range unit.Minis {
		if mini.Removed {
			count++
		}
	}
	return count
}

func activeFrontRankCount(unit Unit) int {
	count := 0
	for _, mini := range unit.Minis {
		if !mini.Removed && mini.Rank == 0 && miniHealth(unit, mini) > 0 {
			count++
		}
	}
	return count
}

func activeFullRanks(unit Unit) int {
	if unit.Base.PerRank <= 0 {
		if activeMiniCount(unit) > 0 {
			return 1
		}
		return 0
	}
	ranks := map[int]int{}
	for _, mini := range unit.Minis {
		if mini.Removed || miniHealth(unit, mini) <= 0 {
			continue
		}
		ranks[mini.Rank]++
	}
	full := 0
	for _, count := range ranks {
		if count >= unit.Base.PerRank {
			full++
		}
	}
	return full
}

func activeLocalBounds(unit Unit) rectBounds {
	box := rectBounds{minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
	for _, mini := range unit.Minis {
		if mini.Removed {
			continue
		}
		box.minX = math.Min(box.minX, mini.RelX)
		box.minY = math.Min(box.minY, mini.RelY)
		box.maxX = math.Max(box.maxX, mini.RelX+float64(mini.WidthMM))
		box.maxY = math.Max(box.maxY, mini.RelY+float64(mini.DepthMM))
	}
	if math.IsInf(box.minX, 1) {
		return rectBounds{}
	}
	return box
}

func unitCenter(unit Unit) (float64, float64) {
	box := unitBoundsAt(unit, unit.X, unit.Y)
	return (box.minX + box.maxX) / 2, (box.minY + box.maxY) / 2
}

func cascadeSourceCenter(unit Unit) (float64, float64) {
	if activeMiniCount(unit) > 0 {
		return unitCenter(unit)
	}
	box := unitAllMiniBoundsAt(unit, unit.X, unit.Y)
	return (box.minX + box.maxX) / 2, (box.minY + box.maxY) / 2
}

func normalizeDeg(deg int) int {
	deg %= 360
	if deg < 0 {
		deg += 360
	}
	return deg
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
