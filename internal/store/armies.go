package store

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"miniswar/internal/game"
)

type CatalogUnit struct {
	ID          string   `json:"id"`
	UnitName    string   `json:"unitName"`
	Nation      string   `json:"nation"`
	Terrain     []string `json:"terrain"`
	A           int      `json:"a"`
	M           int      `json:"m"`
	F           int      `json:"f"`
	S           int      `json:"s"`
	D           int      `json:"d"`
	CD          int      `json:"cd"`
	H           int      `json:"h"`
	Pts         int      `json:"pts"`
	Base        string   `json:"base"`
	BaseWidthMM int      `json:"baseWidthMm"`
	BaseDepthMM int      `json:"baseDepthMm"`
	Special     []string `json:"special"`
	Equipment   []string `json:"equipment"`
}

type ArmyTemplate struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	TargetPoints int                `json:"targetPoints"`
	TotalPoints  int                `json:"totalPoints"`
	Units        []ArmyTemplateUnit `json:"units,omitempty"`
	CreatedAt    string             `json:"createdAt"`
	UpdatedAt    string             `json:"updatedAt"`
}

type ArmyTemplateUnit struct {
	ID             string      `json:"id"`
	TemplateID     string      `json:"templateId"`
	CatalogUnitID  string      `json:"catalogUnitId"`
	DefaultMoniker string      `json:"defaultMoniker"`
	MiniCount      int         `json:"miniCount"`
	SortOrder      int         `json:"sortOrder"`
	CatalogUnit    CatalogUnit `json:"catalogUnit"`
}

type Army struct {
	ID           string     `json:"id"`
	TemplateID   string     `json:"templateId,omitempty"`
	Name         string     `json:"name"`
	TargetPoints int        `json:"targetPoints"`
	TotalPoints  int        `json:"totalPoints"`
	Units        []ArmyUnit `json:"units,omitempty"`
	CreatedAt    string     `json:"createdAt"`
	UpdatedAt    string     `json:"updatedAt"`
}

type ArmyUnit struct {
	ID            string      `json:"id"`
	ArmyID        string      `json:"armyId"`
	CatalogUnitID string      `json:"catalogUnitId"`
	Moniker       string      `json:"moniker"`
	MiniCount     int         `json:"miniCount"`
	MaxHealth     int         `json:"maxHealth"`
	CurrentHealth int         `json:"currentHealth"`
	SortOrder     int         `json:"sortOrder"`
	CatalogUnit   CatalogUnit `json:"catalogUnit"`
}

type catalogFileUnit struct {
	UnitName  string   `json:"Unit Name"`
	Nation    string   `json:"Nation"`
	Terrain   []string `json:"Terrain"`
	A         int      `json:"A"`
	M         int      `json:"M"`
	F         int      `json:"F"`
	S         int      `json:"S"`
	D         int      `json:"D"`
	CD        int      `json:"CD"`
	H         int      `json:"H"`
	Pts       int      `json:"Pts"`
	Special   []string `json:"Special"`
	Base      string   `json:"Base"`
	Equipment []string `json:"Equipment"`
}

func (s *Store) ImportCatalog(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var units []catalogFileUnit
	if err := json.Unmarshal(b, &units); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, unit := range units {
		id := slug(unit.Nation + "-" + unit.UnitName)
		width, depth, err := parseBase(unit.Base)
		if err != nil {
			return fmt.Errorf("%s: %w", unit.UnitName, err)
		}
		special, _ := json.Marshal(unit.Special)
		equipment, _ := json.Marshal(unit.Equipment)
		hash := sha1.Sum(mustJSON(unit))
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err = tx.Exec(`
insert into catalog_units(id, unit_name, nation, a, m, f, s, d, cd, h, pts, base, base_width_mm, base_depth_mm, special_json, equipment_json, source_hash, created_at, updated_at)
values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
on conflict(id) do update set
  unit_name = excluded.unit_name,
  nation = excluded.nation,
  a = excluded.a,
  m = excluded.m,
  f = excluded.f,
  s = excluded.s,
  d = excluded.d,
  cd = excluded.cd,
  h = excluded.h,
  pts = excluded.pts,
  base = excluded.base,
  base_width_mm = excluded.base_width_mm,
  base_depth_mm = excluded.base_depth_mm,
  special_json = excluded.special_json,
  equipment_json = excluded.equipment_json,
  source_hash = excluded.source_hash,
  updated_at = excluded.updated_at`,
			id, unit.UnitName, unit.Nation, unit.A, unit.M, unit.F, unit.S, unit.D, unit.CD, unit.H, unit.Pts, unit.Base, width, depth, string(special), string(equipment), hex.EncodeToString(hash[:]), now, now)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`delete from catalog_unit_terrains where unit_id = ?`, id); err != nil {
			return err
		}
		for _, terrain := range unit.Terrain {
			if _, err := tx.Exec(`insert or ignore into catalog_unit_terrains(unit_id, terrain) values (?, ?)`, id, terrain); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) CatalogUnits(nation, terrain string) ([]CatalogUnit, error) {
	query := `
select distinct cu.id, cu.unit_name, cu.nation, cu.a, cu.m, cu.f, cu.s, cu.d, cu.cd, cu.h, cu.pts, cu.base, cu.base_width_mm, cu.base_depth_mm, cu.special_json, cu.equipment_json
from catalog_units cu
left join catalog_unit_terrains cut on cut.unit_id = cu.id
where (? = '' or cu.nation = ?)
  and (? = '' or exists (select 1 from catalog_unit_terrains t where t.unit_id = cu.id and t.terrain = ?))
order by cu.unit_name collate nocase`
	rows, err := s.db.Query(query, nation, nation, terrain, terrain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCatalogUnits(rows, s)
}

func (s *Store) CatalogFilters() (map[string][]string, error) {
	nations, err := listStrings(s.db, `select distinct nation from catalog_units order by nation collate nocase`)
	if err != nil {
		return nil, err
	}
	terrains, err := listStrings(s.db, `select distinct terrain from catalog_unit_terrains order by terrain collate nocase`)
	if err != nil {
		return nil, err
	}
	return map[string][]string{"nations": nations, "terrains": terrains}, nil
}

func (s *Store) CreateArmyTemplate(name string, targetPoints int) (ArmyTemplate, error) {
	id := newID("tpl")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`insert into army_templates(id, name, target_points, created_at, updated_at) values (?, ?, ?, ?, ?)`, id, cleanName(name, "New Template"), max(0, targetPoints), now, now)
	if err != nil {
		return ArmyTemplate{}, err
	}
	return s.GetArmyTemplate(id)
}

func (s *Store) ListArmyTemplates() ([]ArmyTemplate, error) {
	rows, err := s.db.Query(`select id, name, target_points, created_at, updated_at from army_templates order by updated_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArmyTemplate
	for rows.Next() {
		var t ArmyTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.TargetPoints, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.TotalPoints, _ = s.templatePoints(t.ID)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetArmyTemplate(id string) (ArmyTemplate, error) {
	var t ArmyTemplate
	err := s.db.QueryRow(`select id, name, target_points, created_at, updated_at from army_templates where id = ?`, id).Scan(&t.ID, &t.Name, &t.TargetPoints, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	t.Units, err = s.templateUnits(id)
	if err != nil {
		return t, err
	}
	t.TotalPoints, _ = s.templatePoints(id)
	return t, nil
}

func (s *Store) UpdateArmyTemplate(id, name string, targetPoints int) (ArmyTemplate, error) {
	_, err := s.db.Exec(`update army_templates set name = ?, target_points = ?, updated_at = ? where id = ?`, cleanName(name, "New Template"), max(0, targetPoints), now(), id)
	if err != nil {
		return ArmyTemplate{}, err
	}
	return s.GetArmyTemplate(id)
}

func (s *Store) AddTemplateUnit(templateID, catalogUnitID, moniker string, miniCount int) (ArmyTemplate, error) {
	unit, err := s.GetCatalogUnit(catalogUnitID)
	if err != nil {
		return ArmyTemplate{}, err
	}
	if moniker == "" {
		moniker = unit.UnitName
	}
	id := newID("tunit")
	order, _ := s.nextOrder("army_template_units", "template_id", templateID)
	_, err = s.db.Exec(`insert into army_template_units(id, template_id, catalog_unit_id, default_moniker, mini_count, sort_order, created_at, updated_at) values (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, templateID, catalogUnitID, moniker, defaultMiniCount(unit, miniCount), order, now(), now())
	if err != nil {
		return ArmyTemplate{}, err
	}
	return s.GetArmyTemplate(templateID)
}

func (s *Store) UpdateTemplateUnit(templateID, id, moniker string, miniCount int) (ArmyTemplate, error) {
	var catalogID string
	if err := s.db.QueryRow(`select catalog_unit_id from army_template_units where id = ? and template_id = ?`, id, templateID).Scan(&catalogID); err != nil {
		return ArmyTemplate{}, err
	}
	unit, err := s.GetCatalogUnit(catalogID)
	if err != nil {
		return ArmyTemplate{}, err
	}
	_, err = s.db.Exec(`update army_template_units set default_moniker = ?, mini_count = ?, updated_at = ? where id = ? and template_id = ?`, cleanName(moniker, unit.UnitName), validMiniCount(unit, miniCount), now(), id, templateID)
	if err != nil {
		return ArmyTemplate{}, err
	}
	return s.GetArmyTemplate(templateID)
}

func (s *Store) DeleteTemplateUnit(templateID, id string) (ArmyTemplate, error) {
	_, err := s.db.Exec(`delete from army_template_units where id = ? and template_id = ?`, id, templateID)
	if err != nil {
		return ArmyTemplate{}, err
	}
	return s.GetArmyTemplate(templateID)
}

func (s *Store) CreateArmyFromTemplate(templateID, name string) (Army, error) {
	t, err := s.GetArmyTemplate(templateID)
	if err != nil {
		return Army{}, err
	}
	if name == "" {
		name = t.Name
	}
	id := newID("army")
	tx, err := s.db.Begin()
	if err != nil {
		return Army{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`insert into armies(id, template_id, name, target_points, created_at, updated_at) values (?, ?, ?, ?, ?, ?)`, id, templateID, name, t.TargetPoints, now(), now()); err != nil {
		return Army{}, err
	}
	for _, tu := range t.Units {
		auID := newID("aunit")
		if _, err := tx.Exec(`insert into army_units(id, army_id, catalog_unit_id, moniker, mini_count, max_health, current_health, sort_order, created_at, updated_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			auID, id, tu.CatalogUnitID, tu.DefaultMoniker, tu.MiniCount, tu.CatalogUnit.H, tu.CatalogUnit.H, tu.SortOrder, now(), now()); err != nil {
			return Army{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Army{}, err
	}
	return s.GetArmy(id)
}

func (s *Store) CreateArmy(name string, targetPoints int) (Army, error) {
	id := newID("army")
	_, err := s.db.Exec(`insert into armies(id, name, target_points, created_at, updated_at) values (?, ?, ?, ?, ?)`, id, cleanName(name, "New Army"), max(0, targetPoints), now(), now())
	if err != nil {
		return Army{}, err
	}
	return s.GetArmy(id)
}

func (s *Store) ListArmies() ([]Army, error) {
	rows, err := s.db.Query(`select id, coalesce(template_id, ''), name, target_points, created_at, updated_at from armies order by updated_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Army
	for rows.Next() {
		var a Army
		if err := rows.Scan(&a.ID, &a.TemplateID, &a.Name, &a.TargetPoints, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.TotalPoints, _ = s.armyPoints(a.ID)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetArmy(id string) (Army, error) {
	var a Army
	err := s.db.QueryRow(`select id, coalesce(template_id, ''), name, target_points, created_at, updated_at from armies where id = ?`, id).Scan(&a.ID, &a.TemplateID, &a.Name, &a.TargetPoints, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return a, err
	}
	a.Units, err = s.armyUnits(id)
	if err != nil {
		return a, err
	}
	a.TotalPoints, _ = s.armyPoints(id)
	return a, nil
}

func (s *Store) UpdateArmy(id, name string, targetPoints int) (Army, error) {
	_, err := s.db.Exec(`update armies set name = ?, target_points = ?, updated_at = ? where id = ?`, cleanName(name, "New Army"), max(0, targetPoints), now(), id)
	if err != nil {
		return Army{}, err
	}
	return s.GetArmy(id)
}

func (s *Store) AddArmyUnit(armyID, catalogUnitID, moniker string, miniCount int) (Army, error) {
	unit, err := s.GetCatalogUnit(catalogUnitID)
	if err != nil {
		return Army{}, err
	}
	if moniker == "" {
		moniker = unit.UnitName
	}
	id := newID("aunit")
	order, _ := s.nextOrder("army_units", "army_id", armyID)
	_, err = s.db.Exec(`insert into army_units(id, army_id, catalog_unit_id, moniker, mini_count, max_health, current_health, sort_order, created_at, updated_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, armyID, catalogUnitID, moniker, defaultMiniCount(unit, miniCount), unit.H, unit.H, order, now(), now())
	if err != nil {
		return Army{}, err
	}
	return s.GetArmy(armyID)
}

func (s *Store) UpdateArmyUnit(armyID, id, moniker string, miniCount, currentHealth int) (Army, error) {
	var catalogID string
	var maxHealth int
	if err := s.db.QueryRow(`select catalog_unit_id, max_health from army_units where id = ? and army_id = ?`, id, armyID).Scan(&catalogID, &maxHealth); err != nil {
		return Army{}, err
	}
	unit, err := s.GetCatalogUnit(catalogID)
	if err != nil {
		return Army{}, err
	}
	if currentHealth < 0 {
		currentHealth = 0
	}
	if currentHealth > maxHealth {
		currentHealth = maxHealth
	}
	_, err = s.db.Exec(`update army_units set moniker = ?, mini_count = ?, current_health = ?, updated_at = ? where id = ? and army_id = ?`, cleanName(moniker, unit.UnitName), validMiniCount(unit, miniCount), currentHealth, now(), id, armyID)
	if err != nil {
		return Army{}, err
	}
	return s.GetArmy(armyID)
}

func (s *Store) DeleteArmyUnit(armyID, id string) (Army, error) {
	_, err := s.db.Exec(`delete from army_units where id = ? and army_id = ?`, id, armyID)
	if err != nil {
		return Army{}, err
	}
	return s.GetArmy(armyID)
}

func (s *Store) GetCatalogUnit(id string) (CatalogUnit, error) {
	rows, err := s.db.Query(`
select id, unit_name, nation, a, m, f, s, d, cd, h, pts, base, base_width_mm, base_depth_mm, special_json, equipment_json
from catalog_units where id = ?`, id)
	if err != nil {
		return CatalogUnit{}, err
	}
	defer rows.Close()
	units, err := scanCatalogUnits(rows, s)
	if err != nil {
		return CatalogUnit{}, err
	}
	if len(units) == 0 {
		return CatalogUnit{}, sql.ErrNoRows
	}
	return units[0], nil
}

func (s *Store) ArmyUnitSetups(armyID string, playerID int) ([]game.UnitSetup, error) {
	army, err := s.GetArmy(armyID)
	if err != nil {
		return nil, err
	}
	out := make([]game.UnitSetup, 0, len(army.Units))
	for _, au := range army.Units {
		out = append(out, game.UnitSetup{
			BaseWidthMM:      au.CatalogUnit.BaseWidthMM,
			BaseDepthMM:      au.CatalogUnit.BaseDepthMM,
			Count:            au.MiniCount,
			Name:             au.Moniker,
			CatalogUnitID:    au.CatalogUnitID,
			ArmyID:           armyID,
			ArmyUnitID:       au.ID,
			MaxHealth:        au.MaxHealth,
			CurrentHealth:    au.CurrentHealth,
			CurrentHealthSet: true,
			Stats: game.UnitStats{
				A: au.CatalogUnit.A, M: au.CatalogUnit.M, F: au.CatalogUnit.F, S: au.CatalogUnit.S,
				D: au.CatalogUnit.D, CD: au.CatalogUnit.CD, H: au.CatalogUnit.H, Pts: au.CatalogUnit.Pts,
			},
		})
	}
	return out, nil
}

func scanCatalogUnits(rows *sql.Rows, s *Store) ([]CatalogUnit, error) {
	var out []CatalogUnit
	for rows.Next() {
		var u CatalogUnit
		var special, equipment string
		if err := rows.Scan(&u.ID, &u.UnitName, &u.Nation, &u.A, &u.M, &u.F, &u.S, &u.D, &u.CD, &u.H, &u.Pts, &u.Base, &u.BaseWidthMM, &u.BaseDepthMM, &special, &equipment); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(special), &u.Special)
		_ = json.Unmarshal([]byte(equipment), &u.Equipment)
		terrains, err := listStrings(s.db, `select terrain from catalog_unit_terrains where unit_id = ? order by terrain collate nocase`, u.ID)
		if err != nil {
			return nil, err
		}
		u.Terrain = terrains
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) templateUnits(templateID string) ([]ArmyTemplateUnit, error) {
	rows, err := s.db.Query(`
select atu.id, atu.template_id, atu.catalog_unit_id, atu.default_moniker, atu.mini_count, atu.sort_order,
  cu.id, cu.unit_name, cu.nation, cu.a, cu.m, cu.f, cu.s, cu.d, cu.cd, cu.h, cu.pts, cu.base, cu.base_width_mm, cu.base_depth_mm, cu.special_json, cu.equipment_json
from army_template_units atu
join catalog_units cu on cu.id = atu.catalog_unit_id
where atu.template_id = ?
order by atu.sort_order, atu.created_at`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArmyTemplateUnit
	for rows.Next() {
		var tu ArmyTemplateUnit
		var special, equipment string
		if err := rows.Scan(&tu.ID, &tu.TemplateID, &tu.CatalogUnitID, &tu.DefaultMoniker, &tu.MiniCount, &tu.SortOrder,
			&tu.CatalogUnit.ID, &tu.CatalogUnit.UnitName, &tu.CatalogUnit.Nation, &tu.CatalogUnit.A, &tu.CatalogUnit.M, &tu.CatalogUnit.F, &tu.CatalogUnit.S, &tu.CatalogUnit.D, &tu.CatalogUnit.CD, &tu.CatalogUnit.H, &tu.CatalogUnit.Pts, &tu.CatalogUnit.Base, &tu.CatalogUnit.BaseWidthMM, &tu.CatalogUnit.BaseDepthMM, &special, &equipment); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(special), &tu.CatalogUnit.Special)
		_ = json.Unmarshal([]byte(equipment), &tu.CatalogUnit.Equipment)
		tu.CatalogUnit.Terrain, _ = listStrings(s.db, `select terrain from catalog_unit_terrains where unit_id = ? order by terrain collate nocase`, tu.CatalogUnitID)
		out = append(out, tu)
	}
	return out, rows.Err()
}

func (s *Store) armyUnits(armyID string) ([]ArmyUnit, error) {
	rows, err := s.db.Query(`
select au.id, au.army_id, au.catalog_unit_id, au.moniker, au.mini_count, au.max_health, au.current_health, au.sort_order,
  cu.id, cu.unit_name, cu.nation, cu.a, cu.m, cu.f, cu.s, cu.d, cu.cd, cu.h, cu.pts, cu.base, cu.base_width_mm, cu.base_depth_mm, cu.special_json, cu.equipment_json
from army_units au
join catalog_units cu on cu.id = au.catalog_unit_id
where au.army_id = ?
order by au.sort_order, au.created_at`, armyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArmyUnit
	for rows.Next() {
		var au ArmyUnit
		var special, equipment string
		if err := rows.Scan(&au.ID, &au.ArmyID, &au.CatalogUnitID, &au.Moniker, &au.MiniCount, &au.MaxHealth, &au.CurrentHealth, &au.SortOrder,
			&au.CatalogUnit.ID, &au.CatalogUnit.UnitName, &au.CatalogUnit.Nation, &au.CatalogUnit.A, &au.CatalogUnit.M, &au.CatalogUnit.F, &au.CatalogUnit.S, &au.CatalogUnit.D, &au.CatalogUnit.CD, &au.CatalogUnit.H, &au.CatalogUnit.Pts, &au.CatalogUnit.Base, &au.CatalogUnit.BaseWidthMM, &au.CatalogUnit.BaseDepthMM, &special, &equipment); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(special), &au.CatalogUnit.Special)
		_ = json.Unmarshal([]byte(equipment), &au.CatalogUnit.Equipment)
		au.CatalogUnit.Terrain, _ = listStrings(s.db, `select terrain from catalog_unit_terrains where unit_id = ? order by terrain collate nocase`, au.CatalogUnitID)
		out = append(out, au)
	}
	return out, rows.Err()
}

func (s *Store) templatePoints(id string) (int, error) {
	return queryInt(s.db, `select coalesce(sum(cu.pts * atu.mini_count), 0) from army_template_units atu join catalog_units cu on cu.id = atu.catalog_unit_id where atu.template_id = ?`, id)
}

func (s *Store) armyPoints(id string) (int, error) {
	return queryInt(s.db, `select coalesce(sum(cu.pts * au.mini_count), 0) from army_units au join catalog_units cu on cu.id = au.catalog_unit_id where au.army_id = ?`, id)
}

func (s *Store) nextOrder(table, column, id string) (int, error) {
	return queryInt(s.db, fmt.Sprintf(`select coalesce(max(sort_order), 0) + 1 from %s where %s = ?`, table, column), id)
}

func listStrings(db *sql.DB, query string, args ...any) ([]string, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

func queryInt(db *sql.DB, query string, args ...any) (int, error) {
	var out int
	err := db.QueryRow(query, args...).Scan(&out)
	return out, err
}

func parseBase(base string) (int, int, error) {
	re := regexp.MustCompile(`(?i)^\s*(\d+)\s*x\s*(\d+)\s*$`)
	m := re.FindStringSubmatch(base)
	if len(m) != 3 {
		return 0, 0, fmt.Errorf("invalid base %q", base)
	}
	var width, depth int
	fmt.Sscanf(m[1], "%d", &width)
	fmt.Sscanf(m[2], "%d", &depth)
	return width, depth, nil
}

func validMiniCount(unit CatalogUnit, count int) int {
	base, ok := game.Base(unit.BaseWidthMM, unit.BaseDepthMM)
	if !ok {
		return 1
	}
	if count < 1 {
		return 1
	}
	if count > base.MaxMinis {
		return base.MaxMinis
	}
	return count
}

func defaultMiniCount(unit CatalogUnit, count int) int {
	if count > 0 {
		return validMiniCount(unit, count)
	}
	base, ok := game.Base(unit.BaseWidthMM, unit.BaseDepthMM)
	if !ok {
		return 1
	}
	return validMiniCount(unit, base.PerRank)
}

func slug(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	dash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func newID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func cleanName(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
