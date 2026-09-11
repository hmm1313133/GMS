package database

// Character DAO (Java MapleCharacter / MapleClient.loadCharacters surface),
// on GORM.

import (
	"context"
	"fmt"
)

// Character is the `characters`-table subset the login server needs
// (Java MapleCharacter.loadCharFromDB / getDefault / saveNewCharToDB subset).
//
// The model is deliberately wider than the CHARLIST read: it also carries the
// columns saveNewCharToDB writes with fixed defaults, so one struct serves
// both directions. Game-side columns arrive with P4/P5; the SQLite DDL
// mirrors the full 079-max2 table so both backends share one schema.
type Character struct {
	ID            int    `gorm:"primaryKey;column:id;autoIncrement"`
	AccountID     int    `gorm:"column:accountid;not null"`
	World         int    `gorm:"column:world;not null"`
	Name          string `gorm:"column:name;not null"`
	Level         int    `gorm:"column:level;not null"`
	Exp           int    `gorm:"column:exp;not null"`
	Str           int    `gorm:"column:str;not null"`
	Dex           int    `gorm:"column:dex;not null"`
	Luk           int    `gorm:"column:luk;not null"`
	Int           int    `gorm:"column:int;not null"` // SQL reserved word
	HP            int    `gorm:"column:hp;not null"`
	MP            int    `gorm:"column:mp;not null"`
	MaxHP         int    `gorm:"column:maxhp;not null"`
	MaxMP         int    `gorm:"column:maxmp;not null"`
	Meso          int    `gorm:"column:meso;not null"`
	Job           int    `gorm:"column:job;not null"`
	SkinColor     int    `gorm:"column:skincolor;not null"`
	Gender        int    `gorm:"column:gender;not null"`
	Fame          int    `gorm:"column:fame;not null"`
	Hair          int    `gorm:"column:hair;not null"`
	Face          int    `gorm:"column:face;not null"`
	AP            int    `gorm:"column:ap;not null"`
	Map           int    `gorm:"column:map;not null"`
	Spawnpoint    int    `gorm:"column:spawnpoint;not null"`
	GM            int    `gorm:"column:gm;not null"`
	BuddyCapacity int    `gorm:"column:buddyCapacity;not null"`
	Subcategory   int    `gorm:"column:subcategory;not null"`

	// Written by saveNewCharToDB with fixed defaults only (not read back by
	// CHARLIST): GORM inserts every field of the struct, so they must be
	// part of the model.
	Sp         string `gorm:"column:sp;not null"`
	HPApUsed   int    `gorm:"column:hpApUsed;not null"`
	Party      int    `gorm:"column:party;not null"`
	BookCover  int    `gorm:"column:monsterbookcover;not null"`
	DojoPoints int    `gorm:"column:dojo_pts;not null"`
	DojoRecord int    `gorm:"column:dojoRecord;not null"`
	Pets       string `gorm:"column:pets;not null"`
	MarriageID int    `gorm:"column:marriageId;not null"`
	CurrentRep int    `gorm:"column:currentrep;not null"`
	TotalRep   int    `gorm:"column:totalrep;not null"`
	Prefix     string `gorm:"column:prefix;not null"`
	MountID    int    `gorm:"column:mountid;not null"`
}

// TableName pins the table name.
func (Character) TableName() string { return "characters" }

// CharacterSlot is the per-(account, world) character slot budget.
type CharacterSlot struct {
	AccID   int `gorm:"column:accid;primaryKey"`
	WorldID int `gorm:"column:worldid;primaryKey"`
	Slots   int `gorm:"column:charslots;not null"`
}

func (CharacterSlot) TableName() string { return "character_slots" }

// GetCharactersByAccount ports Java MapleClient.loadCharactersInternal:
// all characters of one account in one world, ordered by id.
//
// GORM quotes every column from the `gorm:"column:..."` tags, which is why
// the reserved-word `int` column no longer needs the per-dialect quoting the
// old sqlx queries carried (db.dialectInt()).
func (db *DB) GetCharactersByAccount(ctx context.Context, accountID, world int) ([]Character, error) {
	var chars []Character
	err := db.WithContext(ctx).
		Where("accountid = ? AND world = ?", accountID, world).
		Order("id").Find(&chars).Error
	if err != nil {
		return nil, fmt.Errorf("database: characters of %d: %w", accountID, err)
	}
	return chars, nil
}

// GetCharacterByID ports Java MapleCharacter.loadCharFromDB's characters-row
// read (SELECT * FROM characters WHERE id = ?): the channel server's player
// assembly (InterServerHandler.Loggedin2). Returns nil, nil when the id does
// not exist.
//
// P4.2 simplification: the Java channelserver path also requires an
// inventoryslot row (and loads skills/quests/rings/... in later statements);
// those tables are not ported yet, so the inventory slot limits fall back to
// saveNewCharToDB's defaults (see internal/packet).
func (db *DB) GetCharacterByID(ctx context.Context, id int) (*Character, error) {
	var c Character
	err := db.WithContext(ctx).Where("id = ?", id).Take(&c).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("database: character %d: %w", id, err)
	}
	return &c, nil
}

// GetCharacterIDByName ports Java MapleCharacterUtil.getIdByName
// (-1 = no such name).
func (db *DB) GetCharacterIDByName(ctx context.Context, name string) (int, error) {
	var c Character
	err := db.WithContext(ctx).Select("id").Where("name = ?", name).Take(&c).Error
	if isNotFound(err) {
		return -1, nil
	}
	if err != nil {
		return -1, fmt.Errorf("database: character id by name %q: %w", name, err)
	}
	return c.ID, nil
}

// InsertCharacter ports Java MapleCharacter.saveNewCharToDB (characters-row
// subset: no queststatus / inventoryitems rows - those tables arrive with
// the P3 inventory layer; new chars show up unequipped in CHARLIST).
// Returns the generated id.
//
// The literals below are saveNewCharToDB's own defaults (empty SP string, no
// pets, party -1); GORM writes them because they are model fields, and
// backfills the auto-increment id into c.
func (db *DB) InsertCharacter(ctx context.Context, c *Character) (int, error) {
	// The old hand-written INSERT never carried `id` (explicit column list);
	// zero it so a caller reusing a struct (or a copy of one that Create has
	// already backfilled) cannot collide on the auto-increment PK.
	c.ID = 0
	c.Sp = "0,0,0,0,0,0,0,0,0,0"
	c.HPApUsed = 0
	c.Party = -1
	c.BookCover = 0
	c.DojoPoints = 0
	c.DojoRecord = 0
	c.Pets = "-1,-1,-1"
	c.MarriageID = 0
	c.CurrentRep = 0
	c.TotalRep = 0
	c.Prefix = ""
	c.MountID = 0

	if err := db.WithContext(ctx).Create(c).Error; err != nil {
		return 0, fmt.Errorf("database: insert character %q: %w", c.Name, err)
	}
	return c.ID, nil
}

// DeleteCharacterByID ports Java MapleClient.deleteCharacter (state 1 =
// no such character for this account / guild leader; 0 = deleted).
// Related rows (inventory/skills/...) cascade via FK in MySQL; the SQLite
// dev backend has no such tables yet - they arrive with the P3 inventory
// layer, so there is nothing to clean there today.
func (db *DB) DeleteCharacterByID(ctx context.Context, id, accountID int) (int, error) {
	// existence + ownership check (Java SELECT guildid, guildrank, familyid)
	var c Character
	err := db.WithContext(ctx).Select("id").
		Where("id = ? AND accountid = ?", id, accountID).Take(&c).Error
	if isNotFound(err) {
		return 1, nil
	}
	if err != nil {
		return 1, fmt.Errorf("database: delete character %d: %w", id, err)
	}
	if err := db.WithContext(ctx).Where("id = ?", id).Delete(&Character{}).Error; err != nil {
		return 1, fmt.Errorf("database: delete character %d: %w", id, err)
	}
	return 0, nil
}

// CharacterSlots ports Java MapleClient.getCharacterSlots: per-account
// world slots, lazily provisioned from def when no row exists.
func (db *DB) CharacterSlots(ctx context.Context, accID, world, def int) (int, error) {
	var slot CharacterSlot
	err := db.WithContext(ctx).
		Where("accid = ? AND worldid = ?", accID, world).Take(&slot).Error
	if isNotFound(err) {
		if err := db.WithContext(ctx).Create(&CharacterSlot{
			AccID: accID, WorldID: world, Slots: def,
		}).Error; err != nil {
			return def, fmt.Errorf("database: insert character_slots %d: %w", accID, err)
		}
		return def, nil
	}
	if err != nil {
		return def, fmt.Errorf("database: character_slots %d: %w", accID, err)
	}
	return slot.Slots, nil
}
