package engine

import (
	"errors"
	"fmt"
	"github.com/299m/udp-dist/messages"
	"github.com/299m/util/util"
	"log"
	"net"
)
import "github.com/299m/udp-dist/routing"

type Config struct {
	ListenOn         []int //// Array of ports to listen on
	TunnelTo         int   //// Port to remotetunnel to and from
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

	remotetunnel *net.UDPConn //// send stuff on the tunnel
	localtunnel  *net.UDPConn //// receive stuff on the tunnel
	tunnelsend   chan *Packet

	localclients map[int64]*net.UDPConn

	config *Config
	quit   bool

	totalrecvd int64

	//// For testing, make this a func ptr
	sendToEndpointFunc func(msgdata []byte, addr *net.UDPAddr)
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

	eng.sendToEndpointFunc = eng.sendToEndpoint //// Can be overridden for UT

	//// if we are local, start all the local listeners
	//// and the remotetunnel listener
	//// and the distributor
	remotetunnel, err := net.Dial("udp", fmt.Sprintf("%s:%d", config.TunnelRemoteAddr, config.TunnelTo))
	util.CheckError(err)
	eng.remotetunnel = remotetunnel.(*net.UDPConn)
	tunnelfromaddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", config.TunnelFrom))
	util.CheckError(err)
	localtunnel, err := net.ListenUDP("udp", tunnelfromaddr)
	eng.localtunnel = localtunnel
	go eng.distribute()
	go eng.processSendToTunnel()
	if eng.islocal {
		fmt.Println("Start local engine, listening on ports", config.ListenOn)
		for _, port := range config.ListenOn {
			addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
			util.CheckError(err)
			go eng.funnelListen(addr)
		}
	}
	return eng

}

func (p *Engine) Close() {
	p.quit = true
	p.tunnelsend <- nil
	p.remotetunnel.Close()
	for _, conn := range p.localclients {
		conn.Close()
	}
}

func (p *Engine) sendToTunnel(message []byte, addr net.Addr) {
	p.tunnelsend <- &Packet{addr: addr.(*net.UDPAddr), data: message}
}

func (p *Engine) processSendToTunnel() {
	fmt.Println("Starting to process packets to remotetunnel")
	for !p.quit {
		packet := <-p.tunnelsend
		if packet == nil {
			break
		}
		fmt.Println("Processing packet to remotetunnel")
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
			/// The send UDP side should have created an association between our recv address and the local (other end of the tunnel) address
			rid, found := p.router.FindRouteByAddr(addr)
			if !found {
				log.Panicln("Failed to find route by addr ", addr)
			}
			localrid, found := p.router.FindAssociation(rid)
			if !found {
				log.Panicln("Failed to find association for ", rid)
			}
			addratlocal := p.router.FindRouteById(localrid)
			fullmsg = p.udpmsg.Write(message, addratlocal)
		}
		x, err := p.remotetunnel.Write(fullmsg)
		util.CheckError(err)
		if x != len(fullmsg) {
			log.Panicln("Failed to write full message", x, len(fullmsg))
		}
	}
}

// // Should be run in a thread to recv on a specific port and send to the remotetunnel
func (p *Engine) funnelListen(from *net.UDPAddr) {
	recvconn, err := net.ListenUDP("udp", from)
	util.CheckError(err)
	defer recvconn.Close()
	fmt.Println("Starting to listen on ", from.String())
	p.funnelRunner(recvconn)
}

func (p *Engine) funnelRunner(recvconn *net.UDPConn) {
	fmt.Println("Enter read loop for ", recvconn.LocalAddr().String())
	for {
		//// let try just using Golang's GC to manage the buffer
		buf := make([]byte, p.recvbufsize)
		fmt.Println("Waiting to read from ", recvconn.LocalAddr().String(), " with ", p.recvbufsize, " buffer")
		n, _, err := recvconn.ReadFrom(buf)
		if errors.Is(err, net.ErrClosed) {
			fmt.Println("*********** Funnel Connection closed ************", recvconn.LocalAddr().String())
			break
		}
		util.CheckError(err)

		fmt.Println("Received ", n, " bytes from ", recvconn.LocalAddr().String())
		/// remotetunnel it
		p.sendToTunnel(buf[:n], recvconn.LocalAddr())
	}
}

func (p *Engine) sendToEndpoint(msgdata []byte, addr *net.UDPAddr) {
	/// Dial a connection for this address if it doesn't exist
	fmt.Println("Find or add route by addr ", addr)

	/// addr is the address from the message
	/// if we are local, it should be the address ot the client to send to
	/// if we are remote, it should be the address of the local client (at the other, local, end of the tunnel)
	rid := p.router.FindOrAddRouteByAddr(addr)
	var err error
	if _, ok := p.localclients[rid]; !ok {
		if !p.islocal {
			remoteaddr := p.config.LocalPortToRemoteAddr[addr.Port]
			raddr, err := net.ResolveUDPAddr("udp", remoteaddr)
			util.CheckError(err)
			addr = raddr
		}
		conn, err := net.DialUDP("udp", nil, addr)
		remoteid := p.router.FindOrAddRouteByAddr(conn.LocalAddr().(*net.UDPAddr))
		p.router.Associate(rid, remoteid)

		///// We must start our listening thread early - or it may miss the response
		go p.funnelRunner(conn)

		util.CheckError(err)
		p.localclients[rid] = conn
	}

	conn := p.localclients[rid]
	fmt.Println("Writing msg to ", conn.RemoteAddr())
	_, err = conn.Write(msgdata)
	util.CheckError(err)
}

// // Read from the remotetunnel and send to the correct port - we may be reading from a TCP connection, so re-find our message boundaries
// / Need to think about how to handle a missed message
func (p *Engine) distribute() {
	/// on the distributor side we have 1 thread reading from the remotetunnel, so the whole
	/// buffer is for this thread
	fmt.Println("Starting distributor, receiving on ", p.localtunnel.LocalAddr())
	buf := make([]byte, p.recvbufsize*p.queuesize)
	offset := 0
	leftover := 0
	for {
		if len(buf)-(offset+leftover) < p.recvbufsize {
			/// We need to shift the buffer
			//fmt.Println("Shift buffer total: ", offset+leftover, ": ", offset, leftover)
			copy(buf, buf[offset+leftover:])
			offset = 0
		}
		//fmt.Println("Waiting to read from tunnel", p.localtunnel.LocalAddr().String())
		n, err := p.localtunnel.Read(buf[offset+leftover:])
		//fmt.Println("Received ", leftover, " bytes from remotetunnel")
		if errors.Is(err, net.ErrClosed) {
			fmt.Println("*********** Dist Connection closed ************")
			break
		}
		util.CheckError(err)
		//// There may be some leftover data from the last message, add the data just read to it
		leftover += n

		//fmt.Println("Leftover is ", leftover)
		for leftover != 0 {
			msgdata, needmore, addr, nextmsgoffset, err := p.udpmsg.Read(buf[offset : offset+leftover])
			util.CheckError(err)
			//fmt.Println("Got a message ", len(msgdata), " from ", addr.String(), " needmore ", needmore, " nextmsgoffset ", nextmsgoffset)

			if needmore {
				/// We need to read more data
				/// We need to read in the next message and append it to the current message
				/// then pass it in again
				break
			}

			p.sendToEndpointFunc(msgdata, addr)

			leftover -= nextmsgoffset
			//fmt.Println("Leftover is now ", leftover, " and nextmsgoffset is ", nextmsgoffset)
			if leftover < 0 {
				log.Panicln("N is negative ", leftover, offset, nextmsgoffset)
			}
			offset += nextmsgoffset
			p.totalrecvd += int64(nextmsgoffset)
			//fmt.Println("Total received ", p.totalrecvd)
		}
	}
}
