package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"GMS/internal/config"
)

func openCharsTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(config.Database{
		Driver:             "sqlite",
		DSN:                filepath.Join(t.TempDir(), "chars.db"),
		MaxOpenConns:       2,
		MaxIdleConns:       1,
		ConnMaxLifetimeSec: 60,
	})
	require.NoError(t, err)
	return db
}

// TestSQLiteCharacterChain covers the whole characters DAO on the
// auto-provisioned SQLite backend: insert (saveNewCharToDB fixed literals),
// CHARLIST readback, name lookup, ownership-checked delete, lazy slot
// provisioning. Guards the GORM migration specifically: the reserved-word
// `int` column and explicit-zero writes (GM=0 must NOT become the column
// default 6, Meso=0 must NOT become 20000).
func TestSQLiteCharacterChain(t *testing.T) {
	db := openCharsTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := &Character{
		AccountID: 7, World: 0,
		Name:  "testchar",
		Level: 5, Exp: 0,
		Str: 12, Dex: 4, Luk: 4, Int: 13, // Int = the reserved-word column
		HP: 50, MP: 20, MaxHP: 50, MaxMP: 20,
		Meso:       0, // explicit zero, not the column default 20000
		Job:        0, SkinColor: 0, Gender: 0, Fame: 0,
		Hair: 13000, Face: 20000,
		AP: 0, Map: 100000000, Spawnpoint: 0,
		GM:            0, // explicit zero, not the column default 6
		BuddyCapacity: 25, Subcategory: 0,
	}
	id, err := db.InsertCharacter(ctx, c)
	require.NoError(t, err)
	require.NotZero(t, id)

	// second char, same account/world: CHARLIST must come back ordered by id
	c2 := *c
	c2.Name = "testchar2"
	id2, err := db.InsertCharacter(ctx, &c2)
	require.NoError(t, err)

	chars, err := db.GetCharactersByAccount(ctx, 7, 0)
	require.NoError(t, err)
	require.Len(t, chars, 2)
	got := chars[0]
	assert.Equal(t, id, got.ID)
	assert.Equal(t, "testchar", got.Name)
	assert.Equal(t, 12, got.Str)
	assert.Equal(t, 4, got.Dex)
	assert.Equal(t, 4, got.Luk)
	assert.Equal(t, 13, got.Int, "reserved-word `int` column broke")
	assert.Equal(t, 0, got.GM, "GM zero became the column default 6")
	assert.Equal(t, 0, got.Meso, "meso zero became the column default 20000")
	assert.Equal(t, 100000000, got.Map)
	assert.Equal(t, 25, got.BuddyCapacity)
	// saveNewCharToDB fixed literals land too (they are model fields now)
	assert.Equal(t, "0,0,0,0,0,0,0,0,0,0", got.Sp)
	assert.Equal(t, "-1,-1,-1", got.Pets)
	assert.Equal(t, -1, got.Party)
	assert.Equal(t, "", got.Prefix)
	// other world: empty
	none, err := db.GetCharactersByAccount(ctx, 7, 1)
	require.NoError(t, err)
	assert.Empty(t, none)

	// name lookup (Java MapleCharacterUtil.getIdByName)
	gotID, err := db.GetCharacterIDByName(ctx, "testchar2")
	require.NoError(t, err)
	assert.Equal(t, id2, gotID)
	missing, err := db.GetCharacterIDByName(ctx, "missing")
	require.NoError(t, err)
	assert.Equal(t, -1, missing)

	// channel-side single-row load (Java MapleCharacter.loadCharFromDB, P4.2)
	one, err := db.GetCharacterByID(ctx, id2)
	require.NoError(t, err)
	require.NotNil(t, one)
	assert.Equal(t, "testchar2", one.Name)
	assert.Equal(t, 7, one.AccountID)
	assert.Equal(t, 100000000, one.Map)
	noneChar, err := db.GetCharacterByID(ctx, 999999)
	require.NoError(t, err)
	assert.Nil(t, noneChar, "unknown id must be nil, not an error")

	// delete: ownership check first (Java deleteCharacter state machine)
	state, err := db.DeleteCharacterByID(ctx, id, 999) // wrong account
	require.NoError(t, err)
	assert.Equal(t, 1, state, "foreign-account delete must report state 1")
	state, err = db.DeleteCharacterByID(ctx, id, 7)
	require.NoError(t, err)
	assert.Equal(t, 0, state)
	chars, err = db.GetCharactersByAccount(ctx, 7, 0)
	require.NoError(t, err)
	require.Len(t, chars, 1)
	assert.Equal(t, id2, chars[0].ID)

	// slots: lazily provisioned from def, then persisted
	slots, err := db.CharacterSlots(ctx, 7, 0, 4)
	require.NoError(t, err)
	assert.Equal(t, 4, slots)
	slots, err = db.CharacterSlots(ctx, 7, 0, 9) // def ignored once a row exists
	require.NoError(t, err)
	assert.Equal(t, 4, slots)
	slots, err = db.CharacterSlots(ctx, 7, 2, 5) // other world provisions its own
	require.NoError(t, err)
	assert.Equal(t, 5, slots)
}

// TestSQLiteAccountAdmin covers the tools/addaccount surface (create / reset
// / delete) and the explicit-zero semantics on accounts: gender=0 must stay
// 0, not become the column default 10.
func TestSQLiteAccountAdmin(t *testing.T) {
	db := openCharsTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// create with gender=0 (male): must NOT become 10 (CHOOSE_GENDER)
	id, err := db.CreateAccount(ctx, "adminacct", sha1hex("pw1"), 0)
	require.NoError(t, err)
	require.NotZero(t, id)
	a, err := db.GetAccountByName(ctx, "adminacct")
	require.NoError(t, err)
	require.NotNil(t, a)
	assert.Equal(t, 0, a.Gender, "gender zero became the column default 10")
	assert.Equal(t, 0, a.Banned)
	assert.Equal(t, 0, a.LoggedIn)
	assert.False(t, a.Salt.Valid)

	// reset: new password, ban/state wiped, NULLs restored
	require.NoError(t, db.WithContext(ctx).Exec(
		"UPDATE accounts SET banned = 1, loggedin = 2, SessionIP = '1.2.3.4' WHERE id = ?", id).Error)
	require.NoError(t, db.ResetAccountPassword(ctx, "adminacct", sha1hex("pw2"), 0))
	a, err = db.GetAccountByName(ctx, "adminacct")
	require.NoError(t, err)
	assert.Equal(t, sha1hex("pw2"), a.Password)
	assert.Equal(t, 0, a.Banned)
	assert.Equal(t, 0, a.LoggedIn)
	assert.False(t, a.SessionIP.Valid)
	assert.False(t, a.Macs.Valid)

	// delete
	n, err := db.DeleteAccountByName(ctx, "adminacct")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	n, err = db.DeleteAccountByName(ctx, "adminacct")
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}
