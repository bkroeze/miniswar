package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

type Engine struct {
	seed int64
	rng  *rand.Rand
}

func NewEngine(seed int64) *Engine {
	return &Engine{seed: seed, rng: rand.New(rand.NewSource(seed))}
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
	default:
		return BaseSize{}, false
	}
}

func (e *Engine) NewGame(setup Setup) (*Game, error) {
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
		unit, err := newUnit(1, playerUnitID(1, i), fmt.Sprintf("Player 1 Unit %d", i+1), unitSetup, 5, 120, 130+i*115, 0)
		if err != nil {
			return nil, fmt.Errorf("player1 unit %d: %w", i+1, err)
		}
		units = append(units, unit)
	}
	for i, unitSetup := range p2Setups {
		unit, err := newUnit(2, playerUnitID(2, i), fmt.Sprintf("Player 2 Unit %d", i+1), unitSetup, 4, 520, 360-i*115, 180)
		if err != nil {
			return nil, fmt.Errorf("player2 unit %d: %w", i+1, err)
		}
		units = append(units, unit)
	}
	first := e.rng.Intn(2) + 1
	return &Game{
		ID:                  fmt.Sprintf("%d-%06d", time.Now().UnixNano(), e.rng.Intn(1000000)),
		Round:               1,
		ActivePlayer:        first,
		FirstPlayer:         first,
		Phase:               "awaiting_activation",
		Units:               units,
		ActionHistory:       []ActionRecord{},
		Snapshots:           []SnapshotRecord{},
		CreatedAt:           time.Now().UTC(),
		RandomSeed:          e.seed,
		OpeningInitiativeD2: first,
	}, nil
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
	if base.WidthMM == 50 && base.DepthMM == 100 && setup.Count != 1 {
		return Unit{}, errors.New("50x100mm bases must be a unit of 1")
	}
	unit := Unit{
		ID:               id,
		PlayerID:         player,
		Name:             name,
		Base:             base,
		ActivationNumber: activation,
		MovementLimitMM:  MovementLimitMM,
		X:                float64(x),
		Y:                float64(y),
		FacingDeg:        normalizeDeg(facing),
	}
	unit.Minis = layoutMinis(unit, setup.Count)
	return unit, nil
}

func layoutMinis(unit Unit, count int) []Mini {
	minis := make([]Mini, 0, count)
	frontRankCount := min(count, unit.Base.PerRank)
	officerFile := max(0, (frontRankCount-1)/2)
	for i := 0; i < count; i++ {
		rank := i / unit.Base.PerRank
		file := i % unit.Base.PerRank
		minis = append(minis, Mini{
			Key:       fmt.Sprintf("p%d-%s-m%02d", unit.PlayerID, unit.ID, i+1),
			UnitID:    unit.ID,
			Index:     i + 1,
			Rank:      rank,
			File:      file,
			RelX:      float64(file * unit.Base.WidthMM),
			RelY:      float64(rank * unit.Base.DepthMM),
			WidthMM:   unit.Base.WidthMM,
			DepthMM:   unit.Base.DepthMM,
			IsOfficer: rank == 0 && file == officerFile,
		})
	}
	return minis
}

func (e *Engine) Activate(g *Game, req ActivateRequest) (*ActionRecord, []int, error) {
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
	if unitActivatedThisRound(g, req.UnitID) {
		return nil, nil, errors.New("unit has already activated this round")
	}
	roll := []int{e.rng.Intn(10) + 1, e.rng.Intn(10) + 1}
	success := roll[0] >= unit.ActivationNumber || roll[1] >= unit.ActivationNumber
	actions := 1
	if success {
		actions = 2
	}
	g.CurrentActivation = &Activation{
		UnitID:           unit.ID,
		PlayerID:         unit.PlayerID,
		Success:          success,
		ActionsRemaining: actions,
		Roll:             roll,
	}
	g.Phase = "activated"
	messages := []string{fmt.Sprintf("%s rolled %d and %d against activation %d", unit.Name, roll[0], roll[1], unit.ActivationNumber)}
	if success {
		messages = append(messages, "Activation succeeded; two actions available.")
	} else {
		messages = append(messages, "Activation failed; one simple action available.")
	}
	rec := g.appendRecord(ActionActivate, req.PlayerID, req.UnitID, req, map[string]any{"success": success, "roll": roll}, messages)
	return &rec, roll, nil
}

func (e *Engine) ApplyAction(g *Game, req ActionRequest) (*ActionRecord, error) {
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
	if act.ActionsRemaining < 1 {
		return nil, errors.New("no actions remaining")
	}

	var messages []string
	switch req.Type {
	case ActionMove:
		if err := applyMove(unit, act, req); err != nil {
			return nil, err
		}
		messages = append(messages, fmt.Sprintf("Moved %s %.0fmm.", req.Direction, req.DistanceMM))
	case ActionPivot:
		anchor, err := pivotAnchor(unit, req.AnchorKey)
		if err != nil {
			return nil, err
		}
		applyPivot(unit, anchor, req.FacingDeg)
		messages = append(messages, fmt.Sprintf("Pivoted to %d degrees around %s.", unit.FacingDeg, anchor.Key))
	case ActionAboutFace:
		if err := applyAboutFace(unit); err != nil {
			return nil, err
		}
		messages = append(messages, "About face completed.")
	default:
		return nil, errors.New("unsupported phase 1 action")
	}

	act.ActionsRemaining--
	if req.Type == ActionMove {
		act.MovesTaken++
	}
	if act.ActionsRemaining == 0 {
		g.CurrentActivation = nil
		g.Phase = "awaiting_activation"
		advanceTurn(g)
	}
	rec := g.appendRecord(req.Type, req.PlayerID, req.UnitID, req, map[string]any{"unit": unit}, messages)
	return &rec, nil
}

func applyMove(unit *Unit, act *Activation, req ActionRequest) error {
	if req.Direction != "forward" && req.Direction != "backward" {
		return errors.New("move direction must be forward or backward")
	}
	limit := float64(unit.MovementLimitMM)
	if req.Direction == "backward" {
		limit = limit / 2
	}
	if act.MovesTaken > 0 {
		limit = limit / 2
	}
	if req.DistanceMM <= 0 || req.DistanceMM > limit {
		return fmt.Errorf("distance must be greater than 0 and no more than %.0fmm", limit)
	}
	sign := 1.0
	if req.Direction == "backward" {
		sign = -1
	}
	rad := float64(unit.FacingDeg) * math.Pi / 180
	unit.X += math.Sin(rad) * req.DistanceMM * sign
	unit.Y -= math.Cos(rad) * req.DistanceMM * sign
	return nil
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

func applyPivot(unit *Unit, anchor Mini, facingDeg int) {
	anchorX, anchorY := miniWorldCenter(*unit, anchor, unit.FacingDeg)
	nextFacing := normalizeDeg(facingDeg)
	relX, relY := rotatePoint(miniCenterX(anchor), miniCenterY(anchor), nextFacing)
	unit.X = anchorX - relX
	unit.Y = anchorY - relY
	unit.FacingDeg = nextFacing
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
	return &g, nil
}

func LegalActions(g *Game) []string {
	if g.CurrentActivation == nil {
		return []string{ActionActivate}
	}
	return []string{ActionMove, ActionPivot, ActionAboutFace}
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
		if unit.PlayerID == playerID && !unitActivatedThisRound(g, unit.ID) {
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
		if !seen[unit.ID] {
			return false
		}
	}
	return true
}

func normalizeDeg(deg int) int {
	deg %= 360
	if deg < 0 {
		deg += 360
	}
	return deg
}
