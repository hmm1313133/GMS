package login

// P2.2 end-to-end login-flow test over a fake account store: full wire
// handshake -> LOGIN_PASSWORD -> LOGIN_STATUS reply, covering every Java
// MapleClient.login branch (ok / banned / wrong pw / no account / double
// login) plus the legacy hash upgrade and gender=10 path.

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"GMS/internal/config"
	"GMS/internal/crypto"
	"GMS/internal/database"
	"GMS/internal/protocol"
)

// fakeStore implements accountStore without a DB.
type fakeStore struct {
	accounts  map[string]*database.Account
	states    map[int]database.AccountState
	updates   map[string]string // side effects: "state:<id>", "pw:<id>", "mac:<id>", "unban:<id>"
	characters []database.Character
	slots     int
	nextCharID int
	// P2.5 ban lists + auto-registration records.
	ipBans    []string
	macBans   map[string]bool
	nextAccID int
	registered []database.Account // accounts created by InsertAutoRegisterAccount
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		accounts:  map[string]*database.Account{},
		states:    map[int]database.AccountState{},
		updates:   map[string]string{},
		slots:     6,
		macBans:   map[string]bool{},
		nextAccID: 100,
	}
}

func (f *fakeStore) GetAccountByName(ctx context.Context, name string) (*database.Account, error) {
	if a, ok := f.accounts[name]; ok {
		cp := *a
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeStore) GetAccountState(ctx context.Context, id int) (database.AccountState, error) {
	return f.states[id], nil
}

func (f *fakeStore) UpdateLoginState(ctx context.Context, id int, newstate int, sessionIP string) error {
	f.updates["state:"+strconvItoa(id)] = strconvItoa(newstate) + "|" + sessionIP
	return nil
}

func (f *fakeStore) UpdatePasswordSHA1(ctx context.Context, id int, password string) error {
	f.updates["pw:"+strconvItoa(id)] = password
	return nil
}

func (f *fakeStore) UnbanAccount(ctx context.Context, id int) error {
	f.updates["unban:"+strconvItoa(id)] = "1"
	return nil
}

func (f *fakeStore) UpdateAccountMac(ctx context.Context, id int, macData string) error {
	f.updates["mac:"+strconvItoa(id)] = macData
	return nil
}

func (f *fakeStore) GetCharactersByAccount(ctx context.Context, accountID, world int) ([]database.Character, error) {
	var out []database.Character
	for i := range f.characters {
		if f.characters[i].AccountID == accountID && f.characters[i].World == world {
			out = append(out, f.characters[i])
		}
	}
	return out, nil
}

func (f *fakeStore) GetCharacterIDByName(ctx context.Context, name string) (int, error) {
	for i := range f.characters {
		if f.characters[i].Name == name {
			return f.characters[i].ID, nil
		}
	}
	return -1, nil
}

func (f *fakeStore) InsertCharacter(ctx context.Context, c *database.Character) (int, error) {
	f.nextCharID++
	c.ID = f.nextCharID
	f.characters = append(f.characters, *c)
	return c.ID, nil
}

func (f *fakeStore) DeleteCharacterByID(ctx context.Context, id, accountID int) (int, error) {
	for i := range f.characters {
		if f.characters[i].ID == id {
			if f.characters[i].AccountID != accountID {
				return 1, nil
			}
			f.characters = append(f.characters[:i], f.characters[i+1:]...)
			return 0, nil
		}
	}
	return 1, nil
}

func (f *fakeStore) CharacterSlots(ctx context.Context, accID, world, def int) (int, error) {
	return f.slots, nil
}

func (f *fakeStore) UpdateAccountGender(ctx context.Context, id int, gender int) error {
	f.updates["gender:"+strconvItoa(id)] = strconvItoa(gender)
	return nil
}

// ---- P2.5 ban list / auto-registration surface ----

func (f *fakeStore) AccountExists(ctx context.Context, name string) (bool, error) {
	_, ok := f.accounts[name]
	return ok, nil
}

func (f *fakeStore) BannedIPs(ctx context.Context) ([]string, error) {
	return f.ipBans, nil
}

func (f *fakeStore) IsBannedMac(ctx context.Context, mac string) (bool, error) {
	return f.macBans[mac], nil
}

func (f *fakeStore) CountAccountsByMac(ctx context.Context, mac string) (int, error) {
	n := 0
	for _, a := range f.accounts {
		if a.Macs.Valid && a.Macs.String == mac {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) InsertAutoRegisterAccount(ctx context.Context, name, passwordSHA1, sessionIP, mac string) error {
	f.nextAccID++
	a := database.Account{
		ID:        f.nextAccID,
		Name:      name,
		Password:  passwordSHA1,
		Gender:    10, // accounts.gender DEFAULT -> next login asks for gender
		Macs:      sql.NullString{String: mac, Valid: true},
		SessionIP: sql.NullString{String: sessionIP, Valid: true},
	}
	f.accounts[name] = &a
	f.states[a.ID] = database.AccountState{LoggedIn: 0}
	f.registered = append(f.registered, a)
	return nil
}

// ResetAccountLogin ports the P4.1 unlockAcc fallback: clear the stale
// loggedin state (both the side-effect record and the live map).
func (f *fakeStore) ResetAccountLogin(ctx context.Context, id int) error {
	f.updates["reset:"+strconvItoa(id)] = "1"
	st := f.states[id]
	st.LoggedIn = database.LoginNotLoggedIn
	f.states[id] = st
	return nil
}

// loginFlow performs the raw-hello handshake, sends one LOGIN_PASSWORD
// packet (login, pwd, 6-byte machine code), and returns the first decrypted
// reply body (LOGIN_STATUS or CHOOSE_GENDER).
func loginFlow(t *testing.T, store accountStore, login, pwd string) []byte {
	t.Helper()
	// AutoRegister off: these tests target the login chain, not P2.5.
	return loginFlowN(t, store, config.Login{Host: "127.0.0.1", Port: 0}, login, pwd, 1)[0]
}

// loginFlowN is the parameterized form of loginFlow: the caller supplies the
// login-server config (P2.5 auto-register switches + ban lists) and how many
// reply frames to read (the auto-register branch answers 2 packets:
// SERVERMESSAGE popup + LOGIN_STATUS).
func loginFlowN(t *testing.T, store accountStore, cfg config.Login, login, pwd string, n int) [][]byte {
	t.Helper()

	lg := slog.New(slog.NewTextHandler(&discardWriter{}, nil))
	srv := New(cfg, lg)
	srv.SetStore(store)
	srv.SetServerName("GMS")
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	addr := srv.acceptor.Addr().String()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	// 1. raw hello: learn both IVs
	body := readN(t, conn, 15)
	recvIV := [4]byte{body[6], body[7], body[8], body[9]}
	sendIV := [4]byte{body[10], body[11], body[12], body[13]}

	// 2. client sends LOGIN_PASSWORD (client codecs: send=recvIV/79, recv=sendIV/-80)
	cSend := crypto.NewAESOFB(recvIV, 79)
	w := protocol.NewWriter(64)
	w.Short(int(protocol.RecvLOGIN_PASSWORD))
	w.MapleAsciiString(login)
	w.MapleAsciiString(pwd)
	w.Write([]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66})
	sendFrame(t, conn, cSend, w.Bytes())

	// 3. read n encrypted reply frames and decrypt
	cRecv := crypto.NewAESOFB(sendIV, -80)
	out := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		hdr := readN(t, conn, 4)
		v := uint32(hdr[0])<<24 | uint32(hdr[1])<<16 | uint32(hdr[2])<<8 | uint32(hdr[3])
		l := v>>16 ^ v&0xFFFF
		m := int(l<<8&0xFF00 | l>>8)
		reply := readN(t, conn, m)
		cRecv.Crypt(reply)
		crypto.ShandaDecrypt(reply)
		out = append(out, reply)
	}
	return out
}

// opOf returns the little-endian send opcode of a decrypted reply body.
func opOf(b []byte) uint16 {
	if len(b) < 2 {
		return 0xFFFF
	}
	return uint16(b[0]) | uint16(b[1])<<8
}

// noticeText extracts the message of a SERVERMESSAGE type-1 popup
// (Java MaplePacketCreator.serverNotice(1, msg)).
func noticeText(t *testing.T, b []byte) string {
	t.Helper()
	if opOf(b) != uint16(protocol.SendSERVERMESSAGE) {
		t.Fatalf("opcode = 0x%04X, want SERVERMESSAGE (0x%04X)", opOf(b), uint16(protocol.SendSERVERMESSAGE))
	}
	if len(b) < 3 || b[2] != 1 {
		t.Fatalf("serverNotice type = %v, want 1", b[2:3])
	}
	r := protocol.NewReader(b[3:])
	return r.MapleAsciiString()
}

// failedReason extracts the reason int of a LOGIN_STATUS / getLoginFailed reply.
func failedReason(t *testing.T, b []byte) int {
	t.Helper()
	if opOf(b) != uint16(protocol.SendLOGIN_STATUS) {
		t.Fatalf("opcode = 0x%04X, want LOGIN_STATUS (0x%04X)", opOf(b), uint16(protocol.SendLOGIN_STATUS))
	}
	if len(b) < 7 {
		t.Fatalf("LOGIN_STATUS truncated: % X", b)
	}
	return int(b[2])
}

// golden constants (testdata/golden_login.txt, produced from the original jar).
const (
	gLegacyHash = "$H$cEEkina27HcGiNraNNQuop.NbVvgRWMlw2Ii="
	gSalt       = "0123456789abcdef0123456789abcdef"
)

func gSha1(s string) string { sum := sha1.Sum([]byte(s)); return hex.EncodeToString(sum[:]) }

func TestLoginFlowOKLegacyUpgrade(t *testing.T) {
	fs := newFakeStore()
	fs.accounts["golduser"] = &database.Account{
		ID: 1, Name: "golduser", Password: gLegacyHash, GM: 0, Gender: 1,
	}
	fs.states[1] = database.AccountState{LoggedIn: 0}

	body := loginFlow(t, fs, "golduser", "goldenpassword")
	if len(body) < 2 || body[0] != 0x00 || body[1] != 0x00 {
		t.Fatalf("opcode = % X, want 00 00 (LOGIN_STATUS)", body[0:2])
	}
	if body[2] != 0 {
		t.Fatalf("success flag = %d, want 0", body[2])
	}
	if v := int(body[3]) | int(body[4])<<8 | int(body[5])<<16 | int(body[6])<<24; v != 1 {
		t.Fatalf("accID = %d, want 1", v)
	}
	// legacy login must upgrade the hash to plain sha1
	if pw, ok := fs.updates["pw:1"]; !ok || pw != gSha1("goldenpassword") {
		t.Fatalf("password upgrade missing: %v", fs.updates)
	}
	// loggedin=2 + SessionIP recorded
	if st, ok := fs.updates["state:1"]; !ok || !strings.HasPrefix(st, "2|") {
		t.Fatalf("login state update missing: %v", fs.updates)
	}
	// machine code persisted
	if mac, ok := fs.updates["mac:1"]; !ok || mac != "11-22-33-44-55-66" {
		t.Fatalf("mac update missing: %v", fs.updates)
	}
}

func TestLoginFlowSHA1OK(t *testing.T) {
	fs := newFakeStore()
	fs.accounts["shauser"] = &database.Account{
		ID: 2, Name: "shauser", Password: gSha1("goldenpassword"), Gender: 1,
	}
	fs.states[2] = database.AccountState{LoggedIn: 0}

	body := loginFlow(t, fs, "shauser", "goldenpassword")
	if body[2] != 0 {
		t.Fatalf("status = %d, want 0 (sha1 ok)", body[2])
	}
	if _, upgraded := fs.updates["pw:2"]; upgraded {
		t.Fatalf("sha1 login must not rewrite the hash: %v", fs.updates)
	}
}

func TestLoginFlowSaltedSHA512Upgrade(t *testing.T) {
	fs := newFakeStore()
	fs.accounts["salted"] = &database.Account{
		ID: 3, Name: "salted",
		Password: makeSaltedSHA512Hash("goldenpassword", gSalt),
		Salt:     sql.NullString{String: gSalt, Valid: true},
		Gender:   1,
	}
	fs.states[3] = database.AccountState{LoggedIn: 0}

	body := loginFlow(t, fs, "salted", "goldenpassword")
	if body[2] != 0 {
		t.Fatalf("status = %d, want 0 (salted sha512 ok)", body[2])
	}
	if pw, ok := fs.updates["pw:3"]; !ok || pw != gSha1("goldenpassword") {
		t.Fatalf("password upgrade missing for salted account: %v", fs.updates)
	}
}

func TestLoginFlowWrongPassword(t *testing.T) {
	fs := newFakeStore()
	fs.accounts["shauser"] = &database.Account{
		ID: 2, Name: "shauser", Password: gSha1("goldenpassword"), Gender: 1,
	}
	fs.states[2] = database.AccountState{LoggedIn: 0}

	body := loginFlow(t, fs, "shauser", "wrongpass")
	if body[2] != 4 {
		t.Fatalf("reason = %d, want 4 (wrong password)", body[2])
	}
}

func TestLoginFlowNoAccount(t *testing.T) {
	fs := newFakeStore()
	body := loginFlow(t, fs, "nobody", "whatever")
	if body[2] != 5 {
		t.Fatalf("reason = %d, want 5 (no account)", body[2])
	}
}

func TestLoginFlowBanned(t *testing.T) {
	fs := newFakeStore()
	fs.accounts["banneduser"] = &database.Account{
		ID: 4, Name: "banneduser", Password: gSha1("goldenpassword"), Banned: 1, Gender: 1,
	}
	fs.states[4] = database.AccountState{LoggedIn: 0}

	body := loginFlow(t, fs, "banneduser", "goldenpassword")
	if body[2] != 3 {
		t.Fatalf("reason = %d, want 3 (banned)", body[2])
	}
}

func TestLoginFlowBannedGMBypass(t *testing.T) {
	fs := newFakeStore()
	fs.accounts["gmbanned"] = &database.Account{
		ID: 5, Name: "gmbanned", Password: gSha1("goldenpassword"), Banned: 1, GM: 1, Gender: 0,
	}
	fs.states[5] = database.AccountState{LoggedIn: 0}

	body := loginFlow(t, fs, "gmbanned", "goldenpassword")
	if body[2] != 0 {
		t.Fatalf("status = %d, want 0 (GM bypasses ban)", body[2])
	}
}

func TestLoginFlowDoubleLogin(t *testing.T) {
	fs := newFakeStore()
	fs.accounts["dupper"] = &database.Account{
		ID: 6, Name: "dupper", Password: gSha1("goldenpassword"), Gender: 1,
	}
	fs.states[6] = database.AccountState{LoggedIn: 2}

	body := loginFlow(t, fs, "dupper", "goldenpassword")
	if body[2] != 7 {
		t.Fatalf("reason = %d, want 7 (already logged in)", body[2])
	}
}

func TestLoginFlowStaleTransition(t *testing.T) {
	fs := newFakeStore()
	fs.accounts["transit"] = &database.Account{
		ID: 7, Name: "transit", Password: gSha1("goldenpassword"), Gender: 1,
	}
	// loggedin=1 (SERVER_TRANSITION) but lastlogin 21s ago -> stale, login proceeds
	fs.states[7] = database.AccountState{
		LoggedIn:  1,
		LastLogin: sql.NullTime{Time: time.Now().Add(-21 * time.Second), Valid: true},
	}

	body := loginFlow(t, fs, "transit", "goldenpassword")
	if body[2] != 0 {
		t.Fatalf("status = %d, want 0 (stale transition)", body[2])
	}
}

func TestLoginFlowGenderNeeded(t *testing.T) {
	fs := newFakeStore()
	// gender=10 (unset) -> CHOOSE_GENDER packet instead of auth success
	fs.accounts["newbie"] = &database.Account{
		ID: 8, Name: "newbie", Password: gSha1("goldenpassword"), Gender: 10,
	}
	fs.states[8] = database.AccountState{LoggedIn: 0}

	body := loginFlow(t, fs, "newbie", "goldenpassword")
	if len(body) < 2 || body[0] != 0x04 || body[1] != 0x00 {
		t.Fatalf("opcode = % X, want 04 00 (CHOOSE_GENDER)", body[0:2])
	}
	nl := int(body[2]) | int(body[3])<<8
	if string(body[4:4+nl]) != "newbie" {
		t.Fatalf("name = %q", body[4:4+nl])
	}
}

// strconvItoa avoids importing strconv twice in this file (keep diff small).
func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
