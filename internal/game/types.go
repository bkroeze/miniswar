package game

import "time"

const (
	ActionActivate       = "activate"
	ActionPlace          = "place"
	ActionMove           = "move"
	ActionPivot          = "pivot"
	ActionAboutFace      = "about_face"
	ActionShoot          = "shoot"
	ActionSkip           = "skip"
	ActionCombatPushback = "combat_pushback"

	CombatChoicePushback25 = "pushback_25"
	CombatChoicePushback75 = "pushback_75"
	CombatChoiceWithdraw25 = "withdraw_25"
	CombatChoiceDecline    = "decline"

	UnitStatusDisordered = "disordered"
	UnitStatusBroken     = "broken"

	CombatFaceFront = "front"
	CombatFaceRear  = "rear"
	CombatFaceRight = "right"
	CombatFaceLeft  = "left"

	MovementLimitMM     = 100
	ArenaWidthMM        = 760
	ArenaHeightMM       = 520
	BrokenMoraleRangeMM = 203.2
	DefaultMiniHealth   = 1

	TerrainRough            = "rough"
	TerrainImpassable       = "impassable"
	TerrainPath             = "path"
	TerrainPassableObstacle = "passable_obstacle"
)

type BaseSize struct {
	WidthMM  int `json:"widthMm"`
	DepthMM  int `json:"depthMm"`
	MaxMinis int `json:"maxMinis"`
	PerRank  int `json:"perRank"`
}

type Setup struct {
	Player1       UnitSetup   `json:"player1"`
	Player2       UnitSetup   `json:"player2"`
	Player1Units  []UnitSetup `json:"player1Units,omitempty"`
	Player2Units  []UnitSetup `json:"player2Units,omitempty"`
	Player1ArmyID string      `json:"player1ArmyId,omitempty"`
	Player2ArmyID string      `json:"player2ArmyId,omitempty"`
	BattlemapID   string      `json:"battlemapId,omitempty"`
	Battlemap     Battlemap   `json:"battlemap,omitempty"`
}

type UnitSetup struct {
	BaseWidthMM      int       `json:"baseWidthMm"`
	BaseDepthMM      int       `json:"baseDepthMm"`
	Count            int       `json:"count"`
	Name             string    `json:"name,omitempty"`
	CatalogUnitID    string    `json:"catalogUnitId,omitempty"`
	ArmyID           string    `json:"armyId,omitempty"`
	ArmyUnitID       string    `json:"armyUnitId,omitempty"`
	MaxHealth        int       `json:"maxHealth,omitempty"`
	CurrentHealth    int       `json:"currentHealth,omitempty"`
	CurrentHealthSet bool      `json:"-"`
	Stats            UnitStats `json:"stats,omitempty"`
	Special          []string  `json:"special,omitempty"`
	Equipment        []string  `json:"equipment,omitempty"`
}

type UnitStats struct {
	A   int `json:"a"`
	M   int `json:"m"`
	F   int `json:"f"`
	S   int `json:"s"`
	D   int `json:"d"`
	CD  int `json:"cd"`
	H   int `json:"h"`
	Pts int `json:"pts"`
}

type Game struct {
	ID                  string               `json:"id"`
	Round               int                  `json:"round"`
	ActivePlayer        int                  `json:"activePlayer"`
	FirstPlayer         int                  `json:"firstPlayer"`
	Phase               string               `json:"phase"`
	WinnerPlayerID      int                  `json:"winnerPlayerId,omitempty"`
	PlacementIndex      int                  `json:"placementIndex"`
	CurrentActivation   *Activation          `json:"currentActivation,omitempty"`
	Units               []Unit               `json:"units"`
	ActionHistory       []ActionRecord       `json:"actionHistory"`
	Snapshots           []SnapshotRecord     `json:"snapshots,omitempty"`
	CreatedAt           time.Time            `json:"createdAt"`
	RandomSeed          int64                `json:"randomSeed"`
	RandomRollIndex     int                  `json:"randomRollIndex"`
	OpeningInitiativeD2 int                  `json:"openingInitiativeD2"`
	Battlemap           Battlemap            `json:"battlemap"`
	Engagements         []CombatEngagement   `json:"engagements,omitempty"`
	PendingCombatChoice *PendingCombatChoice `json:"pendingCombatChoice,omitempty"`
}

type Battlemap struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	WidthMM   float64       `json:"widthMm"`
	HeightMM  float64       `json:"heightMm"`
	Terrains  []TerrainZone `json:"terrains"`
	IsBuiltin bool          `json:"isBuiltin,omitempty"`
}

type TerrainZone struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Label  string  `json:"label"`
	Shape  string  `json:"shape"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Unit struct {
	ID               string    `json:"id"`
	PlayerID         int       `json:"playerId"`
	Name             string    `json:"name"`
	CatalogUnitID    string    `json:"catalogUnitId,omitempty"`
	ArmyID           string    `json:"armyId,omitempty"`
	ArmyUnitID       string    `json:"armyUnitId,omitempty"`
	MaxHealth        int       `json:"maxHealth,omitempty"`
	CurrentHealth    int       `json:"currentHealth,omitempty"`
	CurrentHealthSet bool      `json:"-"`
	Stats            UnitStats `json:"stats,omitempty"`
	Special          []string  `json:"special,omitempty"`
	Equipment        []string  `json:"equipment,omitempty"`
	Base             BaseSize  `json:"base"`
	ActivationNumber int       `json:"activationNumber"`
	MovementLimitMM  int       `json:"movementLimitMm"`
	X                float64   `json:"x"`
	Y                float64   `json:"y"`
	FacingDeg        int       `json:"facingDeg"`
	Placed           bool      `json:"placed"`
	Disordered       bool      `json:"disordered,omitempty"`
	Broken           bool      `json:"broken,omitempty"`
	Minis            []Mini    `json:"minis"`
}

type Mini struct {
	Key             string  `json:"key"`
	UnitID          string  `json:"unitId"`
	Index           int     `json:"index"`
	Rank            int     `json:"rank"`
	File            int     `json:"file"`
	RelX            float64 `json:"relX"`
	RelY            float64 `json:"relY"`
	WidthMM         int     `json:"widthMm"`
	DepthMM         int     `json:"depthMm"`
	IsOfficer       bool    `json:"isOfficer"`
	HealthRemaining int     `json:"healthRemaining,omitempty"`
	Removed         bool    `json:"removed,omitempty"`
}

type CombatEngagement struct {
	ID                 string  `json:"id"`
	AttackerUnitID     string  `json:"attackerUnitId"`
	DefenderUnitID     string  `json:"defenderUnitId"`
	DefenderFace       string  `json:"defenderFace"`
	DefenderFortified  bool    `json:"defenderFortified,omitempty"`
	AxisDX             float64 `json:"axisDx"`
	AxisDY             float64 `json:"axisDy"`
	Round              int     `json:"round"`
	CreatedActionIndex int     `json:"createdActionIndex"`
	Active             bool    `json:"active"`
}

type PendingCombatChoice struct {
	EngagementID      string   `json:"engagementId"`
	WinningPlayerID   int      `json:"winningPlayerId"`
	WinningUnitID     string   `json:"winningUnitId"`
	WinningIsAttacker bool     `json:"winningIsAttacker"`
	LosingUnitID      string   `json:"losingUnitId"`
	Choices           []string `json:"choices"`
	AxisDX            float64  `json:"axisDx"`
	AxisDY            float64  `json:"axisDy"`
	SourceActionIndex int      `json:"sourceActionIndex"`
}

type CombatRoundResult struct {
	EngagementID  string               `json:"engagementId"`
	Attacker      CombatSideResult     `json:"attacker"`
	Defender      CombatSideResult     `json:"defender"`
	Casualties    []CasualtyResult     `json:"casualties,omitempty"`
	MoraleTests   []MoraleTestResult   `json:"moraleTests,omitempty"`
	BrokenUnits   []string             `json:"brokenUnits,omitempty"`
	WinnerUnitID  string               `json:"winnerUnitId,omitempty"`
	PendingChoice *PendingCombatChoice `json:"pendingChoice,omitempty"`
}

type CombatSideResult struct {
	UnitID       string           `json:"unitId"`
	PlayerID     int              `json:"playerId"`
	ContactFace  string           `json:"contactFace"`
	DiceCount    int              `json:"diceCount"`
	TargetNumber int              `json:"targetNumber"`
	Modifiers    []CombatModifier `json:"modifiers"`
	Rolls        []int            `json:"rolls"`
	Hits         int              `json:"hits"`
}

type CombatModifier struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type MoveResult struct {
	Status         string             `json:"status"`
	DistanceMM     float64            `json:"distanceMm"`
	DefenderUnitID string             `json:"defenderUnitId,omitempty"`
	DefenderFace   string             `json:"defenderFace,omitempty"`
	EngagementID   string             `json:"engagementId,omitempty"`
	Combat         *CombatRoundResult `json:"combat,omitempty"`
}

type ShootResult struct {
	TargetUnitID  string             `json:"targetUnitId"`
	Weapon        string             `json:"weapon"`
	RangeMM       float64            `json:"rangeMm"`
	RangeLimitMM  float64            `json:"rangeLimitMm"`
	LOS           LineOfSightResult  `json:"los"`
	DiceCount     int                `json:"diceCount"`
	TargetNumber  int                `json:"targetNumber"`
	Modifiers     []CombatModifier   `json:"modifiers"`
	Rolls         []int              `json:"rolls"`
	Hits          int                `json:"hits"`
	Casualties    []CasualtyResult   `json:"casualties,omitempty"`
	MoraleTests   []MoraleTestResult `json:"moraleTests,omitempty"`
	BrokenUnits   []string           `json:"brokenUnits,omitempty"`
	TargetRemoved bool               `json:"targetRemoved,omitempty"`
}

type LineOfSightResult struct {
	OK              bool     `json:"ok"`
	TargetFacing    string   `json:"targetFacing,omitempty"`
	RequiredFigures int      `json:"requiredFigures,omitempty"`
	VisibleFigures  int      `json:"visibleFigures,omitempty"`
	VisibleMiniKeys []string `json:"visibleMiniKeys,omitempty"`
	Indirect        bool     `json:"indirect,omitempty"`
	BlockedBy       string   `json:"blockedBy,omitempty"`
}

type LegalAction struct {
	Type    string        `json:"type"`
	Targets []ShootTarget `json:"targets,omitempty"`
}

type ShootTarget struct {
	UnitID       string            `json:"unitId"`
	Weapon       string            `json:"weapon"`
	RangeMM      float64           `json:"rangeMm"`
	RangeLimitMM float64           `json:"rangeLimitMm"`
	LOS          LineOfSightResult `json:"los"`
	Center       Position          `json:"center"`
}

type CasualtyResult struct {
	UnitID       string `json:"unitId"`
	MiniKey      string `json:"miniKey"`
	Damage       int    `json:"damage"`
	HealthBefore int    `json:"healthBefore"`
	HealthAfter  int    `json:"healthAfter"`
	Removed      bool   `json:"removed"`
	WasOfficer   bool   `json:"wasOfficer"`
}

type CombatChoiceResult struct {
	Choice              string   `json:"choice"`
	MovingUnitID        string   `json:"movingUnitId,omitempty"`
	RequestedDistanceMM float64  `json:"requestedDistanceMm,omitempty"`
	MovedDistanceMM     float64  `json:"movedDistanceMm,omitempty"`
	AxisDX              float64  `json:"axisDx,omitempty"`
	AxisDY              float64  `json:"axisDy,omitempty"`
	Start               Position `json:"start,omitempty"`
	End                 Position `json:"end,omitempty"`
	StoppedBy           string   `json:"stoppedBy,omitempty"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type MoraleTestResult struct {
	UnitID       string           `json:"unitId"`
	Rolls        []int            `json:"rolls"`
	TargetNumber int              `json:"targetNumber"`
	Modifiers    []CombatModifier `json:"modifiers"`
	Passed       bool             `json:"passed"`
	Cascade      bool             `json:"cascade"`
	Outcome      string           `json:"outcome"`
}

type Activation struct {
	UnitID                string `json:"unitId"`
	PlayerID              int    `json:"playerId"`
	Success               bool   `json:"success"`
	LimitedToSimpleAction bool   `json:"limitedToSimpleAction,omitempty"`
	ActionsRemaining      int    `json:"actionsRemaining"`
	MovesTaken            int    `json:"movesTaken"`
	ShotsTaken            int    `json:"shotsTaken"`
	Roll                  []int  `json:"roll"`
}

type ActionRecord struct {
	Index     int       `json:"index"`
	Round     int       `json:"round"`
	Type      string    `json:"type"`
	PlayerID  int       `json:"playerId"`
	UnitID    string    `json:"unitId,omitempty"`
	Request   any       `json:"request,omitempty"`
	Result    any       `json:"result,omitempty"`
	Messages  []string  `json:"messages"`
	CreatedAt time.Time `json:"createdAt"`
}

type SnapshotRecord struct {
	Index     int       `json:"index"`
	GameJSON  string    `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}

type APIResponse struct {
	OK                 bool          `json:"ok"`
	Game               *Game         `json:"game,omitempty"`
	Games              []GameSummary `json:"games,omitempty"`
	Action             *ActionRecord `json:"action,omitempty"`
	Roll               []int         `json:"roll,omitempty"`
	LegalActions       []string      `json:"legalActions,omitempty"`
	// LegalActionDetails carries target data for actions that need structured choices, such as shooting.
	LegalActionDetails []LegalAction `json:"legalActionDetails,omitempty"`
	// ReadOnly marks historical step responses that clients should not mutate.
	ReadOnly bool     `json:"readOnly,omitempty"`
	Messages []string `json:"messages"`
	Errors   []string `json:"errors,omitempty"`
}

type GameSummary struct {
	ID            string `json:"id"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
	Round         int    `json:"round"`
	Phase         string `json:"phase"`
	ActivePlayer  int    `json:"activePlayer"`
	BattlemapID   string `json:"battlemapId"`
	Battlemap     string `json:"battlemap"`
	ActionCount   int    `json:"actionCount"`
	SnapshotCount int    `json:"snapshotCount"`
}

type ActivateRequest struct {
	PlayerID int    `json:"playerId"`
	UnitID   string `json:"unitId"`
}

type PlacementRequest struct {
	PlayerID  int     `json:"playerId"`
	UnitID    string  `json:"unitId"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	FacingDeg *int    `json:"facingDeg,omitempty"`
}

type ActionRequest struct {
	PlayerID     int     `json:"playerId"`
	UnitID       string  `json:"unitId"`
	Type         string  `json:"type"`
	Direction    string  `json:"direction,omitempty"`
	DistanceMM   float64 `json:"distanceMm,omitempty"`
	FacingDeg    int     `json:"facingDeg,omitempty"`
	AnchorKey    string  `json:"anchorKey,omitempty"`
	CombatChoice string  `json:"combatChoice,omitempty"`
	TargetUnitID string  `json:"targetUnitId,omitempty"`
}

type RewindRequest struct {
	ActionIndex int `json:"actionIndex"`
}
