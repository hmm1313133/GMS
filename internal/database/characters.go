package database

import (
	"context"
	"errors"
	"fmt"
)

// Character is the `characters`-table subset the login server needs
// (Java MapleCharacter.loadCharFromDB / getDefault / saveNewCharToDB subset).
// Game-side columns arrive with P4/P5; the SQLite DDL mirrors the full
// 079-max2 table so both backends share one schema.
type Character struct {
	ID             int    `db:"id"`
	AccountID      int    `db:"accountid"`
	World          int    `db:"world"`
	Name           string `db:"name"`
	Level          int    `db:"level"`
	Exp            int    `db:"exp"`
	Str            int    `db:"str"`
	Dex            int    `db:"dex"`
	Luk            int    `db:"luk"`
	Int            int    `db:"int"`
	HP             int    `db:"hp"`
	MP             int    `db:"mp"`
	MaxHP          int    `db:"maxhp"`
	MaxMP          int    `db:"maxmp"`
	Meso           int    `db:"meso"`
	Job            int    `db:"job"`
	SkinColor      int    `db:"skincolor"`
	Gender         int    `db:"gender"`
	Fame           int    `db:"fame"`
	Hair           int    `db:"hair"`
	Face           int    `db:"face"`
	AP             int    `db:"ap"`
	Map            int    `db:"map"`
	Spawnpoint     int    `db:"spawnpoint"`
	GM             int    `db:"gm"`
	BuddyCapacity  int    `db:"buddyCapacity"`
	Subcategory    int    `db:"subcategory"`
}

// charListQuery loads the CHARLIST columns. `int` is a reserved-ish name in
// both dialects (MySQL dump uses backticks; SQLite identifiers quoted with
// double quotes) - backticks work in MySQL and are rejected by SQLite, so
// this query is built per-driver via charIntCol.
const charListCols = `id, accountid, world, name, level, exp, str, dex, luk, %s,
	hp, mp, maxhp, maxmp, meso, job, skincolor, gender, fame, hair, face, ap,
	map, spawnpoint, gm, buddyCapacity, subcategory`

// GetCharactersByAccount ports Java MapleClient.loadCharactersInternal:
// all characters of one account in one world, ordered by id.
func (db *DB) GetCharactersByAccount(ctx context.Context, accountID, world int) ([]Character, error) {
	q := fmt.Sprintf(`SELECT `+charListCols+` FROM characters
		WHERE accountid = ? AND world = ? ORDER BY id`, db.dialectInt())
	var chars []Character
	if err := db.SelectContext(ctx, &chars, q, accountID, world); err != nil {
		return nil, fmt.Errorf("database: characters of %d: %w", accountID, err)
	}
	return chars, nil
}

// GetCharacterIDByName ports Java MapleCharacterUtil.getIdByName
// (-1 = no such name).
func (db *DB) GetCharacterIDByName(ctx context.Context, name string) (int, error) {
	var id int
	err := db.GetContext(ctx, &id, `SELECT id FROM characters WHERE name = ? LIMIT 1`, name)
	if errors.Is(err, sqlErrNoRows()) {
		return -1, nil
	}
	if err != nil {
		return -1, fmt.Errorf("database: character id by name %q: %w", name, err)
	}
	return id, nil
}

// InsertCharacter ports Java MapleCharacter.saveNewCharToDB (characters-row
// subset: no queststatus / inventoryitems rows - those tables arrive with
// the P3 inventory layer; new chars show up unequipped in CHARLIST).
// Returns the generated id.
func (db *DB) InsertCharacter(ctx context.Context, c *Character) (int, error) {
	q := fmt.Sprintf(`INSERT INTO characters
		(level, fame, str, dex, luk, %s, exp, hp, mp, maxhp, maxmp, sp, ap,
		 gm, skincolor, gender, job, hair, face, map, meso, hpApUsed,
		 spawnpoint, party, buddyCapacity, monsterbookcover, dojo_pts,
		 dojoRecord, pets, subcategory, marriageId, currentrep, totalrep,
		 prefix, accountid, name, world, mountid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		db.dialectInt())
	res, err := db.ExecContext(ctx, q,
		c.Level, c.Fame, c.Str, c.Dex, c.Luk, c.Int, c.Exp, c.HP, c.MP,
		c.MaxHP, c.MaxMP, "0,0,0,0,0,0,0,0,0,0", c.AP, c.GM, c.SkinColor,
		c.Gender, c.Job, c.Hair, c.Face, c.Map, c.Meso, 0, c.Spawnpoint,
		-1, c.BuddyCapacity, 0, 0, 0, "-1,-1,-1", c.Subcategory, 0, 0, 0,
		0, c.AccountID, c.Name, c.World, 0)
	if err != nil {
		return 0, fmt.Errorf("database: insert character %q: %w", c.Name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("database: insert character %q: no id: %w", c.Name, err)
	}
	return int(id), nil
}

// DeleteCharacterByID ports Java MapleClient.deleteCharacter (state 1 =
// no such character for this account / guild leader; 0 = deleted).
// Related rows (inventory/skills/...) cascade via FK in MySQL; the SQLite
// dev backend has no such tables yet - they arrive with the P3 inventory
// layer, so there is nothing to clean there today.
func (db *DB) DeleteCharacterByID(ctx context.Context, id, accountID int) (int, error) {
	// existence + ownership check (Java SELECT guildid, guildrank, familyid)
	var one int
	err := db.GetContext(ctx, &one,
		`SELECT 1 FROM characters WHERE id = ? AND accountid = ? LIMIT 1`, id, accountID)
	if errors.Is(err, sqlErrNoRows()) {
		return 1, nil
	}
	if err != nil {
		return 1, fmt.Errorf("database: delete character %d: %w", id, err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM characters WHERE id = ?`, id); err != nil {
		return 1, fmt.Errorf("database: delete character %d: %w", id, err)
	}
	return 0, nil
}

// CharacterSlots ports Java MapleClient.getCharacterSlots: per-account
// world slots, lazily provisioned from def when no row exists.
func (db *DB) CharacterSlots(ctx context.Context, accID, world, def int) (int, error) {
	var slots int
	err := db.GetContext(ctx, &slots,
		`SELECT charslots FROM character_slots WHERE accid = ? AND worldid = ?`, accID, world)
	if errors.Is(err, sqlErrNoRows()) {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO character_slots (accid, worldid, charslots) VALUES (?, ?, ?)`,
			accID, world, def); err != nil {
			return def, fmt.Errorf("database: insert character_slots %d: %w", accID, err)
		}
		return def, nil
	}
	if err != nil {
		return def, fmt.Errorf("database: character_slots %d: %w", accID, err)
	}
	return slots, nil
}

// sqlErrNoRows is sql.ErrNoRows (named to keep imports small here).
func sqlErrNoRows() error { return sqlNoRows }
