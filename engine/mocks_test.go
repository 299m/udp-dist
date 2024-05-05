package engine

import "net"

type MockEngine struct {
	incoming chan *Packet
}

func NewMockEngine() *MockEngine {
	return &MockEngine{
		incoming: make(chan *Packet, 200),
	}
}

func (m *MockEngine) sendToEndpointFunc(msgdata []byte, addr *net.UDPAddr) {
	m.incoming <- &Packet{addr, msgdata}
}
