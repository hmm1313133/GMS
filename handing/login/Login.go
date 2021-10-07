package login

import (
	"GMS/tools/data/input"
	"bytes"
	"encoding/hex"
	"github.com/panjf2000/gnet"
	"github.com/panjf2000/gnet/pool/goroutine"
	"log"
	"math/rand"
)

type Server struct {
	ServerName string
	Ip         string
	Port       int64
	*gnet.EventServer
	pool *goroutine.Pool
}

func (s *Server) RunStartupConfigurations() {

	p := goroutine.Default()
	defer p.Release()

	echo := new(Server)
	log.Fatal(gnet.Serve(echo, "tcp://:8484", gnet.WithMulticore(true)))
}

func (s *Server) React(frame []byte, c gnet.Conn) (out []byte, action gnet.Action) {
	data := append([]byte{}, frame...)

	// Use ants pool to unblock the event-loop.
	/*_ = s.pool.Submit(func() {
		time.Sleep(1 * time.Second)
		c.AsyncWrite(data)
	})*/

	// 该部分为Java版 MapleServerHandler 中的 messageReceived 方法
	slea := input.NewGenericSeekableLittleEndianAccessor(bytes.NewReader(data), input.NewByteArrayByteStream(data))
	if slea.Available() < 2 {
		return
	}
	return
}

func (s *Server) OnInitComplete(srv gnet.Server) (action gnet.Action) {
	log.Printf("Echo server is listening on %s (multi-cores: %t, loops: %d)\n",
		srv.Addr.String(), srv.Multicore, srv.NumEventLoop)
	return
}

func (s *Server) OnOpened(c gnet.Conn) (out []byte, action gnet.Action) {
	log.Println(c.RemoteAddr().String())

	//send := []byte{82,48,79,245}
	serverRecv := []byte{70, 114, 122, byte(rand.Intn(255))}
	serverSend := []byte{82, 48, 120, byte(rand.Intn(255))}
	c.AsyncWrite(getHello(79, serverSend, serverRecv))

	return
}

func getHello(mapleVersion int, sendIv []byte, recvIv []byte) (packet []byte) {

	packet = append(packet, []byte{13, 0}...)
	packet = append(packet, byte(mapleVersion))
	packet = append(packet, []byte{0, 0}...)
	packet = append(packet, sendIv...)
	packet = append(packet, recvIv...)
	packet = append(packet, 4)
	log.Println(hex.EncodeToString(packet))
	return
}
