package messages

import (
	"bytes"
	"encoding/binary"
	"github.com/299m/util/util"
	"log"
	"net"
)

const (
	ISUDP  = 1
	ISIPV4 = 1 << 1
	ISIPV6 = 1 << 2

	IPADDRSIZE = 16
)

type ErrNotUdp struct{}

func (e ErrNotUdp) Error() string {
	return "Not a UDP packet"
}

// / For UDP we need to convey additional info - so put a header in the packet that goes down the tunnel
type TunnelMessage struct {
	udpheader []byte
	buf       *bytes.Buffer
	nilip     []byte
}

func NewTunnelMessage(bufsize int) *TunnelMessage {
	return &TunnelMessage{
		udpheader: make([]byte, getHeaderSize()), /// 8 bytes for the IP and 4 bytes for the port and 2 bytes for additional info
		buf:       bytes.NewBuffer(make([]byte, bufsize)),
		nilip:     make([]byte, IPADDRSIZE),
	}
}

func (t *TunnelMessage) putUdpHeader(addr *net.UDPAddr, datasize int) {
	t.buf.Reset()
	info := int16(ISUDP)
	addrsize := IPADDRSIZE //// fixed size header
	if addr.IP != nil && addr.IP.To4() == nil {
		info = int16(ISUDP | ISIPV6)
	} else if len(addr.IP) == net.IPv4len {
		info = int16(ISUDP | ISIPV4)
	}

	headersize := binary.Size(info)
	binary.LittleEndian.PutUint16(t.udpheader, uint16(info))
	copy(t.udpheader[headersize:], t.nilip)
	copy(t.udpheader[headersize:], addr.IP)
	headersize += addrsize
	port := uint32(addr.Port)
	binary.LittleEndian.PutUint32(t.udpheader[headersize:], port)
	headersize += binary.Size(port)
	binary.LittleEndian.PutUint32(t.udpheader[headersize:], uint32(datasize))
	t.buf.Write(t.udpheader)
}

func getHeaderSize() int {
	headersize := binary.Size(int16(0))
	headersize += IPADDRSIZE //// fixed size header
	headersize += binary.Size(uint32(0))
	headersize += binary.Size(uint32(0))
	return headersize
}

func (t *TunnelMessage) retrieveUdpHeader(data []byte) (addr *net.UDPAddr, msgdata []byte,
	needmore bool, nextmsgoffset int, err error) {
	if len(data) < getHeaderSize() {
		///// Not an error - just wait for more data and try again
		return nil, nil, true, 0, nil
	}
	headersize := 0
	info := binary.LittleEndian.Uint16(data)
	headersize += binary.Size(info)
	if info&ISUDP == 0 {
		return nil, nil, false, 0, ErrNotUdp{}
	}
	addrsize := IPADDRSIZE
	ipaddr := make([]byte, addrsize)
	copy(ipaddr, data[headersize:headersize+addrsize])
	headersize += addrsize

	port := binary.LittleEndian.Uint32(data[headersize:])
	headersize += binary.Size(port)
	size := binary.LittleEndian.Uint32(data[headersize:])
	headersize += binary.Size(size)

	addr = &net.UDPAddr{
		Port: int(port),
	}
	if info&uint16(ISIPV4) != 0 {
		addr.IP = ipaddr[:net.IPv4len]
	} else if info&uint16(ISIPV6) != 0 {
		addr.IP = ipaddr
	}
	needmore = len(data) < headersize+int(size)
	msgdata = data[headersize : headersize+int(size)]
	nextmsgoffset = headersize + int(size)
	return
}

func (t *TunnelMessage) Write(data []byte, from *net.UDPAddr) (fullmsg []byte) {
	if from == nil {
		log.Panic("Nil address in tunnel write")
	}
	t.putUdpHeader(from, len(data))
	t.buf.Write(data)
	return t.buf.Bytes()
}

// // If need more is set, read in more data and pass this current data + the new data in again
func (t *TunnelMessage) Read(data []byte) (msgdata []byte, needmore bool, addr *net.UDPAddr, nextmsgoffset int, err error) {
	addr, msgdata, needmore, nextmsgoffset, err = t.retrieveUdpHeader(data)
	util.CheckError(err)
	return msgdata, needmore, addr, nextmsgoffset, err
}
