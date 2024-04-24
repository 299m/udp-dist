package engine

import (
	"github.com/299m/util/util"
	"log"
	"net"
	"udp-dist/messages"
	"udp-dist/relay"
)
import "udp-dist/routing"

type Config struct {
	ListenOn   []int //// Array of ports to listen on
	TunnelTo   int   //// Port to tunnel to and from
	TunnelFrom int
}

type Packet struct {
	addr *net.UDPAddr
	data []byte
}

type Engine struct {
	islocal bool
	router  *routing.UdpRoutes
	udpmsg  *messages.TunnelMessage

	recvbufsize int

	tunnel *net.UDPConn

	tunnelsend chan *Packet
}

func NewEngine(north relay.UdpRelay, south relay.UdpRelay, bufsize int, queusize int) *Engine {
	return &Engine{
		router:      routing.NewUdpRoutes(),
		udpmsg:      messages.NewTunnelMessage(bufsize),
		recvbufsize: bufsize,
	}
}

func (p *Engine) sendToTunnel(message []byte, addr net.Addr) {
	p.tunnelsend <- &Packet{addr: addr.(*net.UDPAddr), data: message}
}

func (p *Engine) processSendToTunnel() {
	for {
		packet := <-p.tunnelsend
		message := packet.data
		addr := packet.addr
		var fullmsg []byte
		if p.islocal {
			/// If local we expect to get the full message - this is the raw input
			/// we need to put a header in to say where the message came from
			fullmsg = p.udpmsg.Write(message, addr)
			p.router.FindOrAddRouteByAddr(addr)
		} else { // REMOTE SIDE
			/// If we are not local, we need to lookup the return addr (the UDP address from the local side) and put that in the message
			/// The send UDP side should have filled this in for us based on sending port
			addratlocal := p.router.FindRouteById(int64(addr.Port))
			fullmsg = p.udpmsg.Write(message, addratlocal)
		}
		x, err := p.tunnel.Write(fullmsg)
		util.CheckError(err)
		if x != len(fullmsg) {
			log.Panicln("Failed to write full message", x, len(fullmsg))
		}
	}
}

// // Should be run in a thread to recv on a specific port and send to the tunnel
func (p *Engine) receiver(from *net.UDPAddr) {
	recvconn, err := net.ListenUDP("udp", from)
	util.CheckError(err)
	defer recvconn.Close()

	for {
		//// let try just using Golang's GC to manage the buffer
		buf := make([]byte, p.recvbufsize)
		n, err := recvconn.Read(buf)
		util.CheckError(err)

		/// tunnel it
		p.sendToTunnel(buf[:n], from)

	}
}

func (p *Engine) sender() {

}

// / Listen on mutliple ports and forward to a single address
func (p *Engine) Funnel(to, from *net.UDPAddr) {

	//// read from many, write to the funnel (we write UDP, but the main focus is to be able to pass it thro a proxy
	//// which will probably be TCP)
	for {
		select {
		case msg := <-p.recvqueue:
			//// This is the main funnel - we need to pass the message
		}
	}
}

// / Listen on a single port and distribute to multiple addresses
func (p *Engine) Distribute() {

}
