package engine

import (
	"errors"
	"fmt"
	"github.com/299m/util/util"
	"log"
	"net"
	"udp-dist/messages"
)
import "udp-dist/routing"

type Config struct {
	ListenOn         []int //// Array of ports to listen on
	TunnelTo         int   //// Port to tunnel to and from
	TunnelRemoteAddr string
	TunnelFrom       int

	LocalPortToRemoteAddr map[int]string

	IsLocal bool
}

func (c *Config) Expand() {

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
	queuesize   int

	tunnel     *net.UDPConn
	tunnelsend chan *Packet

	localclients map[int64]*net.UDPConn

	config *Config
	quit   bool
}

func NewEngine(bufsize int, queusize int, config *Config) *Engine {
	eng := &Engine{
		router:       routing.NewUdpRoutes(),
		udpmsg:       messages.NewTunnelMessage(bufsize),
		recvbufsize:  bufsize,
		queuesize:    queusize,
		tunnelsend:   make(chan *Packet, queusize),
		localclients: make(map[int64]*net.UDPConn),
		islocal:      config.IsLocal,
		config:       config,
	}
	//// if we are local, start all the local listeners
	//// and the tunnel listener
	//// and the distributor
	tunnel, err := net.Dial("udp", fmt.Sprintf("%s:%d", config.TunnelRemoteAddr, config.TunnelTo))
	util.CheckError(err)
	eng.tunnel = tunnel.(*net.UDPConn)
	go eng.distribute()
	go eng.processSendToTunnel()
	if eng.islocal {
		fmt.Println("Start local engine, listening on ports", config.ListenOn)
		for _, port := range config.ListenOn {
			addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
			util.CheckError(err)
			go eng.funnel(addr)
		}
	}
	return eng

}

func (p *Engine) Close() {
	p.quit = true
	p.tunnelsend <- nil
	p.tunnel.Close()
	for _, conn := range p.localclients {
		conn.Close()
	}
}

func (p *Engine) sendToTunnel(message []byte, addr net.Addr) {
	p.tunnelsend <- &Packet{addr: addr.(*net.UDPAddr), data: message}
}

func (p *Engine) processSendToTunnel() {
	fmt.Println("Starting to process packets to tunnel")
	for !p.quit {
		packet := <-p.tunnelsend
		if packet == nil {
			break
		}
		fmt.Println("Processing packet to tunnel")
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
func (p *Engine) funnel(from *net.UDPAddr) {
	recvconn, err := net.ListenUDP("udp", from)
	util.CheckError(err)
	defer recvconn.Close()
	fmt.Println("Starting to listen on ", from.String())

	for {
		//// let try just using Golang's GC to manage the buffer
		buf := make([]byte, p.recvbufsize)
		n, err := recvconn.Read(buf)
		if errors.Is(err, net.ErrClosed) {
			fmt.Println("*********** Funnel Connection closed ************", from.String())
			break
		}
		util.CheckError(err)

		fmt.Println("Received ", n, " bytes from ", from.String())
		/// tunnel it
		p.sendToTunnel(buf[:n], from)
	}
}

func (p *Engine) sendToEndpoint(msgdata []byte, addr *net.UDPAddr) (raddr *net.UDPAddr) {
	/// Dial a connection for this address if it doesn't exist
	rid := p.router.FindOrAddRouteByAddr(addr)
	var err error
	if _, ok := p.localclients[rid]; !ok {
		if !p.islocal {
			remoteaddr := p.config.LocalPortToRemoteAddr[addr.Port]
			raddr, err = net.ResolveUDPAddr("udp", remoteaddr)
			util.CheckError(err)
			addr = raddr
		}
		conn, err := net.DialUDP("udp", nil, addr)
		util.CheckError(err)
		p.localclients[rid] = conn
	}
	conn := p.localclients[rid]
	_, err = conn.Write(msgdata)
	util.CheckError(err)
	return raddr
}

// // Read from the tunnel and send to the correct port
func (p *Engine) distribute() {
	////on the distributor side we have 1 thread reading from the tunnel, so the whole
	/// buffer is for this thread
	buf := make([]byte, p.recvbufsize*p.queuesize)
	offset := 0
	for {
		if len(buf)-offset < p.recvbufsize {
			/// We need to shift the buffer
			copy(buf, buf[offset:])
			offset = 0
		}
		n, err := p.tunnel.Read(buf[offset:])
		if errors.Is(err, net.ErrClosed) {
			fmt.Println("*********** Dist Connection closed ************")
			break
		}
		util.CheckError(err)

		for n != 0 {
			msgdata, needmore, addr, nextmsgoffset, err := p.udpmsg.Read(buf[offset : offset+n])
			util.CheckError(err)

			if needmore {
				/// We need to read more data
				/// We need to read in the next message and append it to the current message
				/// then pass it in again
				break
			}
			raddr := p.sendToEndpoint(msgdata, addr)
			if raddr != nil { /// Should only be set if we are remote and this is a new connection
				go p.funnel(raddr)
			}
			n -= nextmsgoffset
			if n < 0 {
				log.Panicln("N is negative ", n, offset, nextmsgoffset)
			}
			offset += nextmsgoffset
		}
	}
}
