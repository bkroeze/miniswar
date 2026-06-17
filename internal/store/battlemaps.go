package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"miniswar/internal/game"
)

func (s *Store) SeedBattlemaps() error {
	for _, battlemap := range game.Battlemaps() {
		game.NormalizeBattlemap(&battlemap)
		if err := game.ValidateBattlemap(battlemap); err != nil {
			return err
		}
		terrainJSON, err := json.Marshal(battlemap.Terrains)
		if err != nil {
			return err
		}
		_, err = s.db.Exec(`
insert into battlemaps(id, name, width_mm, height_mm, terrain_json, is_builtin, created_at, updated_at)
values (?, ?, ?, ?, ?, 1, ?, ?)
on conflict(id) do update set
  name = excluded.name,
  width_mm = excluded.width_mm,
  height_mm = excluded.height_mm,
  terrain_json = excluded.terrain_json,
  is_builtin = 1,
  updated_at = excluded.updated_at`,
			battlemap.ID, battlemap.Name, battlemap.WidthMM, battlemap.HeightMM, string(terrainJSON), now(), now())
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListBattlemaps() ([]game.Battlemap, error) {
	rows, err := s.db.Query(`select id, name, width_mm, height_mm, terrain_json, is_builtin from battlemaps order by is_builtin desc, name collate nocase`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []game.Battlemap
	for rows.Next() {
		battlemap, err := scanBattlemap(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, battlemap)
	}
	return out, rows.Err()
}

func (s *Store) GetBattlemap(id string) (game.Battlemap, error) {
	row := s.db.QueryRow(`select id, name, width_mm, height_mm, terrain_json, is_builtin from battlemaps where id = ?`, id)
	return scanBattlemap(row)
}

func (s *Store) CreateBattlemap(battlemap game.Battlemap) (game.Battlemap, error) {
	if battlemap.ID == "" {
		battlemap.ID = newID("map")
	}
	battlemap.Name = cleanName(battlemap.Name, "New Battlemap")
	game.NormalizeBattlemap(&battlemap)
	if err := game.ValidateBattlemap(battlemap); err != nil {
		return game.Battlemap{}, err
	}
	terrainJSON, err := json.Marshal(battlemap.Terrains)
	if err != nil {
		return game.Battlemap{}, err
	}
	_, err = s.db.Exec(`insert into battlemaps(id, name, width_mm, height_mm, terrain_json, is_builtin, created_at, updated_at) values (?, ?, ?, ?, ?, 0, ?, ?)`,
		battlemap.ID, battlemap.Name, battlemap.WidthMM, battlemap.HeightMM, string(terrainJSON), now(), now())
	if err != nil {
		return game.Battlemap{}, err
	}
	return s.GetBattlemap(battlemap.ID)
}

func (s *Store) UpdateBattlemap(id string, battlemap game.Battlemap) (game.Battlemap, error) {
	existing, err := s.GetBattlemap(id)
	if err != nil {
		return game.Battlemap{}, err
	}
	if existing.IsBuiltin {
		return game.Battlemap{}, fmt.Errorf("battlemap %q is built in and cannot be updated", id)
	}
	battlemap.ID = id
	battlemap.Name = cleanName(battlemap.Name, existing.Name)
	game.NormalizeBattlemap(&battlemap)
	if err := game.ValidateBattlemap(battlemap); err != nil {
		return game.Battlemap{}, err
	}
	terrainJSON, err := json.Marshal(battlemap.Terrains)
	if err != nil {
		return game.Battlemap{}, err
	}
	_, err = s.db.Exec(`update battlemaps set name = ?, width_mm = ?, height_mm = ?, terrain_json = ?, updated_at = ? where id = ?`,
		battlemap.Name, battlemap.WidthMM, battlemap.HeightMM, string(terrainJSON), now(), id)
	if err != nil {
		return game.Battlemap{}, err
	}
	return s.GetBattlemap(id)
}

func (s *Store) DeleteBattlemap(id string) error {
	var isBuiltin int
	err := s.db.QueryRow(`select is_builtin from battlemaps where id = ?`, id).Scan(&isBuiltin)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}
	if isBuiltin == 1 {
		return fmt.Errorf("battlemap %q is built in and cannot be deleted", id)
	}
	_, err = s.db.Exec(`delete from battlemaps where id = ?`, id)
	return err
}

type battlemapScanner interface {
	Scan(dest ...any) error
}

func scanBattlemap(scanner battlemapScanner) (game.Battlemap, error) {
	var battlemap game.Battlemap
	var terrainJSON string
	var isBuiltin int
	if err := scanner.Scan(&battlemap.ID, &battlemap.Name, &battlemap.WidthMM, &battlemap.HeightMM, &terrainJSON, &isBuiltin); err != nil {
		return battlemap, err
	}
	if err := json.Unmarshal([]byte(terrainJSON), &battlemap.Terrains); err != nil {
		return battlemap, err
	}
	game.NormalizeBattlemap(&battlemap)
	battlemap.IsBuiltin = isBuiltin == 1
	return battlemap, nil
}
