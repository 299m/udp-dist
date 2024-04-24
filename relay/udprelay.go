package relay

import (
	"github.com/299m/util/logging"
	"github.com/299m/util/util"
	"log"
	"net"
	"time"
)

type UdpRelay struct {
	udpconn   *net.UDPConn
	buff      []byte
	debuglogs logging.DebugLog
	connid    string

	url     string
	timeout time.Duration
}

func NewUdpRelay(udpconn *net.UDPConn, bufsize int) *UdpRelay {
	return &UdpRelay{
		udpconn: udpconn,
		buff:    make([]byte, bufsize),
	}
}

func NewUdpClient(url string, timeout time.Duration) *UdpRelay {
	return &UdpRelay{
		url:     url,
		timeout: timeout,
	}
}

func (u *UdpRelay) EnableDebugLogs(enabled bool, connid string) {
	u.debuglogs.EnableDebugLogs(enabled, connid)
	u.connid = connid
}

func (u *UdpRelay) Listen(addr *net.UDPAddr) {
	conn, err := net.ListenUDP("udp", addr)
	util.CheckError(err)
	u.udpconn = conn
}

func (u *UdpRelay) Connect() error {
	if u.udpconn != nil {
		log.Panicln("trying to connect a UDP relay that is already connected")
	}
	addr, err := net.ResolveUDPAddr("udp", u.url)
	util.CheckError(err)
	conn, err := net.DialUDP("udp", nil, addr)
	util.CheckError(err)
	u.udpconn = conn
	return nil
}

func (u *UdpRelay) RecvMsg() ([]byte, net.Addr, error) {
	n, addr, err := u.udpconn.ReadFromUDP(u.buff)
	if err != nil {
		u.debuglogs.LogDebug("Error reading from UDP"+err.Error(), u.connid)
		return nil, nil, err
	}
	u.debuglogs.LogDebug("Received UDP packet from "+addr.String(), u.connid)
	return u.buff[:n], addr, nil
}

func (u *UdpRelay) SendMsg(data []byte) error {
	_, err := u.udpconn.Write(data)
	if err != nil {
		u.debuglogs.LogDebug("Error writing to UDP"+err.Error(), u.connid)
		return err
	}
	u.debuglogs.LogDebug("Sent UDP packet", u.connid)
	return nil
}

func (u *UdpRelay) Close() {
	u.udpconn.Close()
}
