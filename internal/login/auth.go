// Package login - P2.2 account/password login flow.
//
// This file ports the Java login chain:
//   - handling/login/handler/CharLoginHandler.login -> handleLoginPassword
//   - client/MapleClient.login / finishLogin / updateLoginState -> (*client).login
//   - handling/login/LoginWorker.registerClient        -> registerClient
//   - handling/login/handler/AutoRegister              -> (P2.5) register.go
//   - client/MapleClient.hasBannedIP / isBannedMac     -> (P2.5) register.go
//
// Simplifications vs the Java original (documented):
//   - The 登陆保护/队列/多开 (login guard / queue / multi-client) config checks
//     are skipped (they are per-distribution add-ons around the core loginok
//     chain, driven by the `configvalues` table which is absent from the
//     079-max2 dump; PLAN classifies them SKIP or P7).

package login

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"GMS/internal/database"
	"GMS/internal/netw"
	"GMS/internal/protocol"
)

// loginFailed reasons (Java MapleClient.login return codes / client LOGIN_STATUS ints).
const (
	loginOK          = 0 // success
	loginBanned      = 3 // banned > 0 and not GM
	loginWrongPw     = 4 // password mismatch
	loginNoAccount   = 5 // no row for that name
	loginAlreadyIn   = 7 // loggedin state > 0 (double login)
)

// accountStore is the DB surface the login flow needs (implemented by
// *database.DB; faked in tests).
type accountStore interface {
	GetAccountByName(ctx context.Context, name string) (*database.Account, error)
	GetAccountState(ctx context.Context, id int) (database.AccountState, error)
	UpdateLoginState(ctx context.Context, id int, newstate int, sessionIP string) error
	UpdatePasswordSHA1(ctx context.Context, id int, password string) error
	UnbanAccount(ctx context.Context, id int) error
	UpdateAccountMac(ctx context.Context, id int, macData string) error

	// P2.4 character-side surface (Java MapleClient.loadCharacters /
	// MapleCharacterUtil / MapleCharacter.saveNewCharToDB / deleteCharacter /
	// getCharacterSlots / updateGender).
	GetCharactersByAccount(ctx context.Context, accountID, world int) ([]database.Character, error)
	GetCharacterIDByName(ctx context.Context, name string) (int, error)
	InsertCharacter(ctx context.Context, c *database.Character) (int, error)
	DeleteCharacterByID(ctx context.Context, id, accountID int) (int, error)
	CharacterSlots(ctx context.Context, accID, world, def int) (int, error)
	UpdateAccountGender(ctx context.Context, id int, gender int) error

	// P2.5 ban lists + auto-registration (MapleClient.hasBannedIP /
	// isBannedMac + AutoRegister.createAccount).
	AccountExists(ctx context.Context, name string) (bool, error)
	BannedIPs(ctx context.Context) ([]string, error)
	IsBannedMac(ctx context.Context, mac string) (bool, error)
	CountAccountsByMac(ctx context.Context, mac string) (int, error)
	InsertAutoRegisterAccount(ctx context.Context, name, passwordSHA1, sessionIP, mac string) error

	// P4.1 double-login cleanup (MapleClient.unlockAcc else-branch).
	ResetAccountLogin(ctx context.Context, id int) error
}

// client is the per-connection login state (Java MapleClient subset).
type client struct {
	accID        int
	accountName  string
	mac          string
	gm           bool
	gender       int
	loginAttempt int
	// loggedIn mirrors Java MapleClient.loggedIn: set true by
	// updateLoginState(LOGIN_LOGGEDIN); gates NeedsChecking opcodes
	// (SERVERLIST/SERVERSTATUS/...).
	loggedIn bool
	// allowedChars is the Java MapleClient.allowedChar set: ids that may be
	// selected/deleted (filled by loadCharacters, checked by login_Auth).
	allowedChars map[int]bool
	// world/channel are the selected world+channel (Java setWorld/setChannel).
	world   int
	channel int
}

// sessionIP extracts the pure IP from a RemoteAddr (Java getSessionIPAddress:
// "/1.2.3.4:5555" -> "/1.2.3.4").
func sessionIP(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = strings.TrimPrefix(remote, "/")
	}
	return host
}

// handleLoginPassword ports CharLoginHandler.login (RecvLOGIN_PASSWORD
// 0x0001): body after the 2-byte opcode = MapleAsciiString login,
// MapleAsciiString pwd, 6 raw machine-code bytes.
func (h handler) handleLoginPassword(s *netw.Session, r *protocol.Reader) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	login := r.MapleAsciiString()
	pwd := r.MapleAsciiString()
	if r.Err != nil || login == "" {
		h.srv.log.Warn("malformed LOGIN_PASSWORD", "remote", s.RemoteAddr)
		s.Write(LoginFailedPacket(loginNoAccount))
		return
	}
	macData := macFromBytes(r.Read(6)) // machine code, informational

	c := &client{accountName: login, mac: macData}
	s.State.Store(c)

	if h.srv.store == nil {
		// Degraded mode (cmd/gms: DB unreachable at startup): answer the
		// stock "no such account" instead of dereferencing a nil store.
		h.srv.log.Warn("login without database", "login", login)
		s.Write(LoginFailedPacket(loginNoAccount))
		return
	}

	// CharLoginHandler.login: the IP/MAC ban checks run before the login
	// chain and before auto-registration.
	ipBan := h.srv.bannedIP(ctx, s.RemoteAddr)
	macBan := h.srv.bannedMac(ctx, macData)
	banned := ipBan || macBan

	// AutoRegister branch: unknown account + not banned -> register instead
	// of authenticating (Java AutoRegister.autoRegister / 账号注册开关).
	if h.srv.cfg.AutoRegister && !banned {
		exists, err := h.srv.store.AccountExists(ctx, login)
		if err != nil {
			h.srv.log.Error("account exists check failed", "err", err, "login", login)
		} else if !exists {
			h.srv.autoRegister(s, login, pwd, macData, s.RemoteAddr)
			return
		}
	}

	loginok, acc, needUpgrade := h.srv.dbLogin(ctx, c, login, pwd)
	c.accID, c.gm, c.gender = acc.ID, acc.GM > 0, acc.Gender

	// Java: if (loginok == 0 && (ipBan || macBan) && !c.isGm()) loginok = 3
	// (a banned machine may still hold valid credentials).
	if loginok == loginOK && banned && !c.gm {
		loginok = loginBanned
	}

	// CharLoginHandler: fail-count guard (>5 attempts stop answering).
	c.loginAttempt++
	if loginok != loginOK {
		if c.loginAttempt > 5 {
			h.srv.log.Warn("login fail-count exceeded", "login", login, "remote", s.RemoteAddr)
			return
		}
		s.Write(LoginFailedPacket(loginok))
		return
	}

	if needUpgrade {
		// Java updatePasswordHashtosha1: upgrade legacy / salted hashes to
		// plain SHA-1 after a successful login.
		if err := h.srv.store.UpdatePasswordSHA1(ctx, acc.ID, hexSHA1(pwd)); err != nil {
			h.srv.log.Error("password upgrade failed", "err", err, "accID", acc.ID)
		}
	}

	// c.updateMacs(): persist the machine code (skip all-zero MACs, Java).
	if macData != "" && macData != zeroMAC {
		if err := h.srv.store.UpdateAccountMac(ctx, acc.ID, macData); err != nil {
			h.srv.log.Error("mac update failed", "err", err, "accID", acc.ID)
		}
	}

	// LoginWorker.registerClient -> finishLogin: set loggedin=2 + SessionIP.
	ip := sessionIP(s.RemoteAddr)
	if err := h.srv.store.UpdateLoginState(ctx, acc.ID, database.LoginLoggedIn, ip); err != nil {
		h.srv.log.Error("login state update failed", "err", err, "accID", acc.ID)
	} else {
		c.loggedIn = true // Java updateLoginState(LOGIN_LOGGEDIN): loggedIn = true
		// P4.1: remember which session holds the account so a later double
		// login can evict it (Java World.Client registration).
		h.srv.registerClient(acc.ID, s)
	}

	if c.gender == 10 { // Java getGenderNeeded for unset-gender accounts
		h.srv.log.Info("login ok, gender needed", "login", login, "accID", acc.ID)
		s.Write(GenderNeededPacket(login))
		return
	}
	h.srv.log.Info("login ok", "login", login, "accID", acc.ID, "gm", c.gm)
	s.Write(AuthSuccessPacket(acc.ID, byte(c.gender), c.gm, login))
}

// dbLogin ports MapleClient.login(name, pwd): the loginok chain.
// Returns (loginok, account, upgradeToSHA1).
func (srv *Server) dbLogin(ctx context.Context, c *client, login, pwd string) (int, *database.Account, bool) {
	acc, err := srv.store.GetAccountByName(ctx, login)
	if err != nil {
		srv.log.Error("account lookup failed", "err", err, "login", login)
		return loginNoAccount, &database.Account{}, false
	}
	if acc == nil {
		return loginNoAccount, &database.Account{}, false
	}

	gm := acc.GM > 0

	// banned handling: >0 and not GM -> 3; ==-1 auto-unban then continue.
	if acc.Banned > 0 && !gm {
		return loginBanned, acc, false
	}
	if acc.Banned == -1 {
		if err := srv.store.UnbanAccount(ctx, acc.ID); err != nil {
			srv.log.Error("unban failed", "err", err, "accID", acc.ID)
		}
	}

	// Double-login: live loggedin state (transition states 1/6 time out
	// after 20s like Java getLoginState).
	state, err := srv.store.GetAccountState(ctx, acc.ID)
	if err != nil {
		srv.log.Error("state lookup failed", "err", err, "accID", acc.ID)
		return loginWrongPw, acc, false
	}
	loggedin := state.LoggedIn
	if loggedin == database.LoginServerTransit || loggedin == database.ChangeChannel {
		if !state.LastLogin.Valid || !withinTransitionWindow(state.LastLogin.Time) {
			loggedin = database.LoginNotLoggedIn
		}
	}
	if loggedin > database.LoginNotLoggedIn {
		// Java: the double-login branch still answers loginok=7, but first
		// unlocks the account (evict the live session, or clear the stale
		// loggedin when the old client is gone) so the retry succeeds.
		srv.unlockAcc(ctx, acc.ID)
		return loginAlreadyIn, acc, false
	}

	// Password chain (Java order): legacy $H$ -> salt==NULL sha1 ->
	// superpw -> salted sha512.
	if isLegacyPassword(acc.Password) && legacyCheckPassword(pwd, acc.Password) {
		return loginOK, acc, true
	}
	if !acc.Salt.Valid && checkSHA1Hash(acc.Password, pwd) {
		return loginOK, acc, false
	}
	if srv.cfg.SuperPassword && pwd != "" && pwd == srv.superpw {
		return loginOK, acc, false
	}
	if acc.Salt.Valid && checkSaltedSHA512Hash(acc.Password, pwd, acc.Salt.String) {
		return loginOK, acc, true
	}
	return loginWrongPw, acc, false
}

// withinTransitionWindow ports the Java 20s connecting-to-chanserver timeout
// (lastlogin + 20000 < now -> treat as logged out).
func withinTransitionWindow(last time.Time) bool {
	if last.IsZero() {
		return false
	}
	return time.Since(last) < 20*time.Second
}

// macFromBytes formats the 6 machine-code bytes Java-style:
// "XX-XX-XX-XX-XX-XX" (uppercase, left-padded hex).
func macFromBytes(b []byte) string {
	if len(b) != 6 {
		return ""
	}
	parts := make([]string, 6)
	for i, v := range b {
		parts[i] = fmt.Sprintf("%02X", v)
	}
	return strings.Join(parts, "-")
}
