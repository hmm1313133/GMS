package login

import (
	"math/rand/v2"
	"net"
	"strconv"
)

// rngInt returns a pseudo-random int in [0, n) for session IV bytes
// (Java: Randomizer.nextInt(255)).
func rngInt(n int) int { return rand.IntN(n) }

func netwJoinHostPort(host string, port int) string {
	if host == "" || host == "0.0.0.0" {
		return ":" + strconv.Itoa(port)
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// writeFull writes b to conn completely (net.Conn.Write may be partial).
func writeFull(conn net.Conn, b []byte) error {
	for len(b) > 0 {
		n, err := conn.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return net.ErrClosed
		}
		b = b[n:]
	}
	return nil
}
