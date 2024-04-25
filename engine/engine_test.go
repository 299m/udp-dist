package engine

import (
	"errors"
	"fmt"
	"github.com/299m/util/util"
	"github.com/stretchr/testify/assert"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
	"udp-dist/messages"
)

const (
	LISTENPORT1 = 8001
	LISTENPORT2 = 8002
	LISTENPORT3 = 8003
)

func setup(local bool) (eng *Engine, mocktunnel *net.UDPConn, mockendpoints []*net.UDPConn) {

	//// Listen on a UDP port as a mock tunnel
	//// Point the engine at this mock tunnel
	//// Add a few listening ports for the engine
	//// Send a message to the mock tunnel
	//// Make sure the message is received by the mcok tunnel and mathces the sent messages
	endpointlisteners := []int{9001, 9002, 9003}
	config := &Config{
		ListenOn:         []int{LISTENPORT1, LISTENPORT2, LISTENPORT3},
		TunnelTo:         8000,
		TunnelRemoteAddr: "localhost",
		TunnelFrom:       9000,
		IsLocal:          local,
	}
	mocktunneladdr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", config.TunnelRemoteAddr, config.TunnelTo))
	util.CheckError(err)
	mocktunnel, err = net.ListenUDP("udp", mocktunneladdr)
	util.CheckError(err)

	mockendpoints = make([]*net.UDPConn, len(config.ListenOn))
	for i, port := range endpointlisteners {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
		util.CheckError(err)
		mockendpoints[i], err = net.ListenUDP("udp", addr)
		util.CheckError(err)
	}
	engine := NewEngine(2048, 100, config)
	time.Sleep(1 * time.Second) /// allow the tunnel to get started
	return engine, mocktunnel, mockendpoints
}

func recv(mocktunnel *net.UDPConn, recvdmsgs chan []byte) {
	for {
		buff := make([]byte, 2048)
		n, _, err := mocktunnel.ReadFromUDP(buff)
		if errors.Is(err, net.ErrClosed) {
			fmt.Println("*********** Mocktunnel Connection closed ************")
			break
		}
		util.CheckError(err)
		recvdmsgs <- buff[:n]
	}
}

func Test_LocalEngineFunnel(t *testing.T) {
	eng, mocktunnel, mockendpoints := setup(true)
	defer mocktunnel.Close()
	defer eng.Close()
	for _, endpoint := range mockendpoints {
		defer endpoint.Close()
	}
	//// Send a message to one of the listening ports and read it from the mock tunnel
	//// listen on the mcok tunnel first
	recvdmsgs := make(chan []byte, 5)
	go recv(mocktunnel, recvdmsgs)
	msgs := make([][]byte, 3)
	msgs[0] = []byte("0 zz12345678901234567890123456789012345678901234567890123456789012345678901234567890aa")
	msgs[1] = []byte("1 xx12345678901234567890123456789012345678901234567890123456789012345678901234567890aa")
	msgs[2] = []byte("2 yy12345678901234567890123456789012345678901234567890123456789012345678901234567890aa")
	fmt.Println("Posting message to ", LISTENPORT1)
	sendaddr := make([]*net.UDPAddr, 3)
	var err error
	sendaddr[0], err = net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", LISTENPORT1))
	util.CheckError(err)
	mockendpoints[0].WriteToUDP(msgs[0], sendaddr[0])

	sendaddr[1], err = net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", LISTENPORT2))
	util.CheckError(err)
	mockendpoints[1].WriteToUDP(msgs[1], sendaddr[1])

	sendaddr[2], err = net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", LISTENPORT3))
	util.CheckError(err)
	mockendpoints[2].WriteToUDP(msgs[2], sendaddr[2])

	tmsghandler := messages.NewTunnelMessage(2048)

	to := time.NewTimer(2 * time.Second)
	for range msgs {
		select {
		case recvdmsg := <-recvdmsgs:
			///check the message has the correct header info
			msgdata, needmore, addr, _, err := tmsghandler.Read(recvdmsg)
			fmt.Println("Received message from ", addr.String(), "data", string(msgdata))
			indxpart := strings.Split(string(msgdata), " ")[0]
			indx, err := strconv.ParseInt(indxpart, 10, 32)
			util.CheckError(err)
			fmt.Println("indx ", indx)
			assert.Nil(t, err, "Error reading message")
			assert.False(t, needmore, "Need more is true")
			assert.Equal(t, sendaddr[indx].String(), addr.String(), "Address mismatch")
			assert.Equal(t, msgs[indx], msgdata, "Data mismatch")
		case <-to.C:
			t.Error("Timed out waiting for message")
		}
	}
}
