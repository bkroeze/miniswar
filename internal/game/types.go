package game

import "time"

const (
	ActionActivate  = "activate"
	ActionMove      = "move"
	ActionPivot     = "pivot"
	ActionAboutFace = "about_face"

	MovementLimitMM = 100
)

type BaseSize struct {
	WidthMM  int `json:"widthMm"`
	DepthMM  int `json:"depthMm"`
	MaxMinis int `json:"maxMinis"`
	PerRank  int `json:"perRank"`
}

type Setup struct {
	Player1      UnitSetup   `json:"player1"`
	Player2      UnitSetup   `json:"player2"`
	Player1Units []UnitSetup `json:"player1Units,omitempty"`
	Player2Units []UnitSetup `json:"player2Units,omitempty"`
}

type UnitSetup struct {
	BaseWidthMM int `json:"baseWidthMm"`
	BaseDepthMM int `json:"baseDepthMm"`
	Count       int `json:"count"`
}

type Game struct {
	ID                  string           `json:"id"`
	Round               int              `json:"round"`
	ActivePlayer        int              `json:"activePlayer"`
	FirstPlayer         int              `json:"firstPlayer"`
	Phase               string           `json:"phase"`
	CurrentActivation   *Activation      `json:"currentActivation,omitempty"`
	Units               []Unit           `json:"units"`
	ActionHistory       []ActionRecord   `json:"actionHistory"`
	Snapshots           []SnapshotRecord `json:"snapshots,omitempty"`
	CreatedAt           time.Time        `json:"createdAt"`
	RandomSeed          int64            `json:"randomSeed"`
	OpeningInitiativeD2 int              `json:"openingInitiativeD2"`
}

type Unit struct {
	ID               string   `json:"id"`
	PlayerID         int      `json:"playerId"`
	Name             string   `json:"name"`
	Base             BaseSize `json:"base"`
	ActivationNumber int      `json:"activationNumber"`
	MovementLimitMM  int      `json:"movementLimitMm"`
	X                float64  `json:"x"`
	Y                float64  `json:"y"`
	FacingDeg        int      `json:"facingDeg"`
	Minis            []Mini   `json:"minis"`
}

type Mini struct {
	Key       string  `json:"key"`
	UnitID    string  `json:"unitId"`
	Index     int     `json:"index"`
	Rank      int     `json:"rank"`
	File      int     `json:"file"`
	RelX      float64 `json:"relX"`
	RelY      float64 `json:"relY"`
	WidthMM   int     `json:"widthMm"`
	DepthMM   int     `json:"depthMm"`
	IsOfficer bool    `json:"isOfficer"`
}

type Activation struct {
	UnitID           string `json:"unitId"`
	PlayerID         int    `json:"playerId"`
	Success          bool   `json:"success"`
	ActionsRemaining int    `json:"actionsRemaining"`
	MovesTaken       int    `json:"movesTaken"`
	Roll             []int  `json:"roll"`
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
	OK           bool          `json:"ok"`
	Game         *Game         `json:"game,omitempty"`
	Action       *ActionRecord `json:"action,omitempty"`
	Roll         []int         `json:"roll,omitempty"`
	LegalActions []string      `json:"legalActions,omitempty"`
	Messages     []string      `json:"messages"`
	Errors       []string      `json:"errors,omitempty"`
}

type ActivateRequest struct {
	PlayerID int    `json:"playerId"`
	UnitID   string `json:"unitId"`
}

type ActionRequest struct {
	PlayerID   int     `json:"playerId"`
	UnitID     string  `json:"unitId"`
	Type       string  `json:"type"`
	Direction  string  `json:"direction,omitempty"`
	DistanceMM float64 `json:"distanceMm,omitempty"`
	FacingDeg  int     `json:"facingDeg,omitempty"`
	AnchorKey  string  `json:"anchorKey,omitempty"`
}

type RewindRequest struct {
	ActionIndex int `json:"actionIndex"`
}
