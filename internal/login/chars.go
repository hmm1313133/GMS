// Package login - P2.4 character list / create / delete / gender.
//
// Java sources (see docs/FILETRACK.md):
//   - CharLoginHandler.CharlistRequest   -> handleCharlistRequest (0x0009)
//   - CharLoginHandler.CheckCharName     -> handleCheckCharName   (0x000C)
//   - CharLoginHandler.CreateChar        -> handleCreateChar      (0x0011)
//   - CharLoginHandler.DeleteChar        -> handleDeleteChar      (0x0012)
//   - CharLoginHandler.SetGenderRequest  -> handleSetGender       (0x0004)
//
// Documented P2.4 simplifications (P3+ will revisit):
//   - New characters are inserted into `characters` only: no
//     queststatus rows (newbie quests 20022/20010/...) and no
//     inventoryitems rows (starter equips + guidebook) - the login DB
//     surface has no inventory tables yet, so charselect shows them
//     unequipped. Layout of the packets is unaffected.
//   - The 20 world/job-class GUI toggles (职业开关) are treated as
//     "all open" (original distribution defaults).
//   - Create-limit uses the character-slot count (character_slots table,
//     default 6 / GM 15) instead of the configvalues `创建角色数量` row
//     (not present in the 079-max2 dump; slot semantics nearly coincide).
//   - Second-password check reuses the login crypto chain; the Java
//     LoginCrypto.rand_r second-password wrapper is not ported (accounts
//     without 2ndpassword - the common case - are fully faithful).
//   - RSA_KEY/STRANGE_DATA and SECONDPW_ERROR opcodes are dead in the
//     original (absent from recv/send.ini, value -2): not ported.
package login

import (
	"context"
	"regexp"
	"time"
	"unicode/utf8"

	"GMS/internal/database"
	"GMS/internal/netw"
	"GMS/internal/protocol"
)

// namePattern ports Java MapleCharacterUtil.namePattern
// ([0-9\u4e00-\u9fa5]{2,5}).
var namePattern = regexp.MustCompile(`^[0-9\x{4e00}-\x{9fa5}]{2,5}$`)

// reservedWords ports GameConstants.RESERVED.
var reservedWords = []string{"Rental"}

// createCharLimit is the default slot count (Java DEFAULT_CHARSLOT=6).
const createCharLimit = 6

// canCreateCharName ports MapleCharacterUtil.canCreateChar: pattern match,
// name unused, eligible (2..15 UTF-16 units, no reserved words).
func canCreateCharName(name string, existing func(string) (bool, error)) bool {
	if !namePattern.MatchString(name) {
		return false
	}
	id, err := existing(name)
	if err != nil || id {
		return false
	}
	// isEligibleCharName
	if n := utf8.RuneCountInString(name); n > 15 || n < 2 {
		return false
	}
	for _, z := range reservedWords {
		if contains(name, z) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// loginClient loads the *client from the session state slot.
func (h handler) loginClient(s *netw.Session) *client {
	c, _ := s.State.Load().(*client)
	return c
}

// handleCharlistRequest ports CharLoginHandler.CharlistRequest:
// byte server + byte channel(+1) + int skip; then setWorld(0)/setChannel,
// loadCharacters(0) and reply getCharList(secondpw != nil, chars, slots).
func (h handler) handleCharlistRequest(s *netw.Session, r *protocol.Reader) {
	c := h.loginClient(s)
	if c == nil || !c.loggedIn {
		return // NeedsChecking gate
	}
	r.Byte() // server (ignored, Java forces setWorld(0))
	channel := int(r.Byte()) + 1
	r.Int() // skipped in Java too
	c.world = 0
	c.channel = channel

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chars, err := h.srv.store.GetCharactersByAccount(ctx, c.accID, c.world)
	if err != nil {
		h.srv.log.Error("load characters failed", "err", err, "accID", c.accID)
		s.Close("load characters failed")
		return
	}
	if c.allowedChars == nil {
		c.allowedChars = map[int]bool{}
	}
	for i := range chars {
		c.allowedChars[chars[i].ID] = true // Java allowedChar.add
	}

	def := createCharLimit
	if c.gm {
		def = 15 // Java getCharacterSlots: GM gets 15
	}
	slots, err := h.srv.store.CharacterSlots(ctx, c.accID, c.world, def)
	if err != nil {
		h.srv.log.Error("character slots failed", "err", err, "accID", c.accID)
		slots = def
	}
	s.Write(CharListPacket(chars, slots))
}

// handleCheckCharName ports CharLoginHandler.CheckCharName: reply
// charNameResponse(name, !canCreateChar || forbidden). The notice packet
// for invalid names is skipped when the name fails canCreateChar (Java
// sends serverNotice + getLoginFailed(1) there too - kept).
func (h handler) handleCheckCharName(s *netw.Session, r *protocol.Reader) {
	c := h.loginClient(s)
	if c == nil || !c.loggedIn {
		return
	}
	name := r.MapleAsciiString()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	used := func(n string) (bool, error) {
		id, err := h.srv.store.GetCharacterIDByName(ctx, n)
		return id != -1, err
	}
	ok := canCreateCharName(name, used)
	if !ok {
		// Java also sends the dialog + getLoginFailed(1) for malformed names.
		s.Write(ServerNoticeDialogPacket("提示：角色名格式不正确，只能中文或者数字。"))
		s.Write(LoginFailedPacket(1))
	}
	s.Write(CharNameResponsePacket(name, !ok))
}

// createCharJob maps JobType to the starting job (Java MapleCharacter.
// getDefault: 1= adventurer 0, 0= knight 1000, 2= aran 2000) and the
// starting map (saveNewCharToDB: adventurer 0, knight 130030000, aran
// 914000000, else 910000000).
func createCharJob(jobType int) (job, startMap int) {
	switch jobType {
	case 1:
		return 0, 0
	case 0:
		return 1000, 130030000
	case 2:
		return 2000, 914000000
	default:
		return 0, 910000000
	}
}

// starter item whitelists (Java CreateChar guards).
var (
	starterShoes  = map[int]bool{1072001: true, 1072005: true, 1072037: true, 1072038: true, 1072383: true}
	starterWeapon = map[int]bool{1302000: true, 1322005: true, 1312004: true, 1442079: true}
)

// handleCreateChar ports CharLoginHandler.CreateChar: body = str name +
// int jobType + int face + int hair + int top + int bottom + int shoes +
// int weapon. Validations: gender 0/1, shoes/weapon whitelists, slot limit;
// then getDefault stats + saveNewCharToDB + addNewCharEntry(worked).
func (h handler) handleCreateChar(s *netw.Session, r *protocol.Reader) {
	c := h.loginClient(s)
	if c == nil || !c.loggedIn {
		return
	}
	name := r.MapleAsciiString()
	jobType := int(r.Int())
	face := int(r.Int())
	hair := int(r.Int())
	r.Int() // top (starter equip, inventory is P3 - parsed for fidelity)
	r.Int() // bottom
	shoes := int(r.Int())
	weapon := int(r.Int())
	if r.Err != nil {
		h.srv.log.Warn("malformed CREATE_CHAR", "err", r.Err, "remote", s.RemoteAddr)
		return
	}

	gender := c.gender
	if gender != 0 && gender != 1 {
		h.srv.log.Warn("create char dropped: bad gender", "gender", gender, "accID", c.accID)
		return // Java silently drops
	}
	if !starterShoes[shoes] || !starterWeapon[weapon] {
		h.srv.log.Warn("create char dropped: item not whitelisted",
			"shoes", shoes, "weapon", weapon, "name", name)
		return // Java silently drops
	}
	job, startMap := createCharJob(jobType)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// slot limit (Java: loadCharacterIds(world).size() >= 创建角色数量)
	chars, err := h.srv.store.GetCharactersByAccount(ctx, c.accID, c.world)
	if err != nil {
		h.srv.log.Error("load characters failed", "err", err, "accID", c.accID)
		s.Write(AddNewCharEntryPacket(&database.Character{Name: name}, false))
		return
	}
	def := createCharLimit
	if c.gm {
		def = 15
	}
	slots, err := h.srv.store.CharacterSlots(ctx, c.accID, c.world, def)
	if err != nil {
		slots = def
	}
	if len(chars) >= slots {
		s.Write(ServerNoticeDialogPacket("无法继续创建新角色。"))
		s.Write(LoginFailedPacket(1))
		return
	}

	// Java MapleCharacter.getDefault stats.
	nc := &database.Character{
		AccountID:     c.accID,
		World:         c.world,
		Name:          name,
		Level:         1,
		Str:           12,
		Dex:           5,
		Int:           4,
		Luk:           4,
		HP:            50,
		MP:            50,
		MaxHP:         50,
		MaxMP:         50,
		Meso:          0,
		Job:           job,
		SkinColor:     0,
		Gender:        gender,
		Fame:          0,
		Hair:          hair,
		Face:          face,
		AP:            0,
		Map:           startMap,
		Spawnpoint:    0,
		GM:            0, // Java: client.gm ? 5 : 0 (login-phase gm flag)
		BuddyCapacity: 20,
		Subcategory:   0,
	}
	if c.gm {
		nc.GM = 5
	}

	used := func(n string) (bool, error) {
		id, err := h.srv.store.GetCharacterIDByName(ctx, n)
		return id != -1, err
	}
	if canCreateCharName(name, used) {
		id, err := h.srv.store.InsertCharacter(ctx, nc)
		if err != nil {
			h.srv.log.Error("insert character failed", "err", err, "name", name)
			s.Write(AddNewCharEntryPacket(&database.Character{Name: name}, false))
			return
		}
		nc.ID = id
		if c.allowedChars == nil {
			c.allowedChars = map[int]bool{}
		}
		c.allowedChars[id] = true // Java c.createdChar
		h.srv.log.Info("character created", "name", name, "id", id, "accID", c.accID)
		s.Write(AddNewCharEntryPacket(nc, true))
	} else {
		s.Write(AddNewCharEntryPacket(&database.Character{Name: name}, false))
	}
}

// handleDeleteChar ports CharLoginHandler.DeleteChar: byte skip + str
// secondpw + int cid. login_Auth (allowedChar set) then second-password
// (NULL = skip) then deleteCharacter -> deleteCharResponse(cid, state).
func (h handler) handleDeleteChar(s *netw.Session, r *protocol.Reader) {
	c := h.loginClient(s)
	if c == nil || !c.loggedIn {
		return
	}
	r.Byte()
	secondpw := r.MapleAsciiString()
	charID := int(r.Int())
	if r.Err != nil {
		return
	}

	// Java login_Auth: only char ids loaded for this account.
	if !c.allowedChars[charID] {
		h.srv.log.Warn("delete unauthorized char", "accID", c.accID, "charID", charID)
		return // Java sends secondPwError(20); opcode is dead in v079 (no send value)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// fetch the account row to get 2ndpassword/salt2
	acc, err := h.srv.store.GetAccountByName(ctx, c.accountName)
	if err != nil || acc == nil {
		s.Close("account lookup failed on delete")
		return
	}

	state := 0
	if acc.SecondPassword.Valid {
		// Java CheckSecondPassword chain (minus the rand_r wrapper):
		// legacy $H$ -> salt2==NULL sha1 -> salted sha512.
		ok := false
		switch {
		case isLegacyPassword(acc.SecondPassword.String) && legacyCheckPassword(secondpw, acc.SecondPassword.String):
			ok = true
		case !acc.Salt2.Valid && checkSHA1Hash(acc.SecondPassword.String, secondpw):
			ok = true
		case acc.Salt2.Valid && checkSaltedSHA512Hash(acc.SecondPassword.String, secondpw, acc.Salt2.String):
			ok = true
		}
		if !ok {
			state = 16
		}
	}
	if state == 0 {
		state, err = h.srv.store.DeleteCharacterByID(ctx, charID, c.accID)
		if err != nil {
			h.srv.log.Error("delete character failed", "err", err, "charID", charID)
			state = 1
		} else {
			delete(c.allowedChars, charID)
		}
	}
	s.Write(DeleteCharResponsePacket(charID, state))
}

// handleSetGender ports CharLoginHandler.SetGenderRequest: byte gender +
// str username. On a matching account name: updateGender + getGenderChanged
// + licenseRequest + updateLoginState(0, ip).
func (h handler) handleSetGender(s *netw.Session, r *protocol.Reader) {
	c := h.loginClient(s)
	if c == nil {
		return // pre-login packet on a fresh session: Java SetGender has
		// NeedsChecking=true, but the gender flow happens right after
		// CHOOSE_GENDER when loggedIn is already set; nil = drop.
	}
	gender := int(r.Byte())
	username := r.MapleAsciiString()
	if username != c.accountName {
		s.Close("set gender name mismatch")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.srv.store.UpdateAccountGender(ctx, c.accID, gender); err != nil {
		h.srv.log.Error("update gender failed", "err", err, "accID", c.accID)
	}
	c.gender = gender
	s.Write(GenderChangedPacket(c.accountName, c.accID))
	s.Write(LicenseRequestPacket())
	// Java updateLoginState(0, ip) -> loggedin reset + loggedIn=false.
	ip := sessionIP(s.RemoteAddr)
	if err := h.srv.store.UpdateLoginState(ctx, c.accID, database.LoginNotLoggedIn, ip); err != nil {
		h.srv.log.Error("login state update failed", "err", err, "accID", c.accID)
	}
	c.loggedIn = false
}
