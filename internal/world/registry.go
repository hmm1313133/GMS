// Package world holds the cross-server shared state that the Java original
// kept in static maps on LoginServer/World (login tickets, the online-player
// finder). In the Go single-binary layout login and channel are separate
// packages, so these live here - both may depend on world, neither depends
// on the other.
//
// Java sources (see docs/FILETRACK.md):
//   - handling/login/LoginServer.loginAuth / loginIPAuth -> LoginRegistry
//   - handling/world/World.Find                         -> Finder
package world

import "sync"

// LoginAuth is one pending login ticket (Java LoginServer.putLoginAuth's
// Triple<ip, tempIp, channel>). Written by the login server on CHAR_SELECT,
// fetched (and removed) when the channel session picks the character up.
type LoginAuth struct {
	// IP is the login session remote in "ip:port" form (Java passes
	// remoteAddress().toString() minus the leading '/').
	IP string
	// TempIP mirrors Java MapleClient.tempIP (empty on the login path; the
	// channel-change flow fills it).
	TempIP string
	// Channel is the selected channel (1-based).
	Channel int
}

// LoginRegistry ports the two static maps on Java LoginServer: loginAuth
// (charId -> ticket) and loginIPAuth (the ip set). The ZEV original never
// actually consumes the tickets channel-side (the containsIPAuth check is an
// empty if-block) - kept here with the same semantics for the P4.2 port.
type LoginRegistry struct {
	mu     sync.Mutex
	auths  map[int]LoginAuth
	ipAuth map[string]bool
}

// NewLoginRegistry returns an empty registry (Java static initialisers).
func NewLoginRegistry() *LoginRegistry {
	return &LoginRegistry{
		auths:  map[int]LoginAuth{},
		ipAuth: map[string]bool{},
	}
}

// PutLoginAuth ports LoginServer.putLoginAuth: remember the ticket and mark
// the ip as authenticated.
func (r *LoginRegistry) PutLoginAuth(charID int, ip, tempIP string, channel int) {
	r.mu.Lock()
	r.auths[charID] = LoginAuth{IP: ip, TempIP: tempIP, Channel: channel}
	r.ipAuth[ip] = true
	r.mu.Unlock()
}

// TakeLoginAuth ports LoginServer.getLoginAuth: fetch-and-remove (one-shot).
func (r *LoginRegistry) TakeLoginAuth(charID int) (LoginAuth, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.auths[charID]
	if ok {
		delete(r.auths, charID)
	}
	return a, ok
}

// ContainsIPAuth ports LoginServer.containsIPAuth.
func (r *LoginRegistry) ContainsIPAuth(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ipAuth[ip]
}

// RemoveIPAuth ports LoginServer.removeIPAuth.
func (r *LoginRegistry) RemoveIPAuth(ip string) {
	r.mu.Lock()
	delete(r.ipAuth, ip)
	r.mu.Unlock()
}

// AddIPAuth ports LoginServer.addIPAuth (the channel-change path re-adds the
// entry).
func (r *LoginRegistry) AddIPAuth(ip string) {
	r.mu.Lock()
	r.ipAuth[ip] = true
	r.mu.Unlock()
}
