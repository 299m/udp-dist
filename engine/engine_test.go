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

var config *Config

func setup(local bool) (eng *Engine, mocktunnel *net.UDPConn, mockendpoints []*net.UDPConn) {

	//// Listen on a UDP port as a mock remotetunnel
	//// Point the engine at this mock remotetunnel
	//// Add a few listening ports for the engine
	//// Send a message to the mock remotetunnel
	//// Make sure the message is received by the mcok remotetunnel and mathces the sent messages
	endpointlisteners := []int{9001, 9002, 9003}
	config = &Config{
		ListenOn:         []int{LISTENPORT1, LISTENPORT2, LISTENPORT3},
		TunnelTo:         8000,
		TunnelRemoteAddr: "localhost",
		TunnelFrom:       9000,
		IsLocal:          local,
		LocalPortToRemoteAddr: map[int]string{
			LISTENPORT1: fmt.Sprintf("localhost:%d", endpointlisteners[0]),
			LISTENPORT2: fmt.Sprintf("localhost:%d", endpointlisteners[1]),
			LISTENPORT3: fmt.Sprintf("localhost:%d", endpointlisteners[2]),
		},
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
	time.Sleep(1 * time.Second) /// allow the remotetunnel to get started
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
	//// Send a message to one of the listening ports and read it from the mock remotetunnel
	//// listen on the mcok remotetunnel first
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

func simulateRemoteClient(conn *net.UDPConn, msgs chan *Packet) {
	//// lookup the remote addr we should be simulating

	for {
		buff := make([]byte, 2048)
		n, err := conn.Read(buff)
		util.CheckError(err)
		fmt.Println("Queuing message from ", conn.LocalAddr().String())
		msgs <- &Packet{
			addr: conn.LocalAddr().(*net.UDPAddr),
			data: buff[:n],
		}
	}
}

func Test_RemoteEngineDistributor(t *testing.T) {
	/// Send some data to mock endpoints, make sure we get it
	/// Then reply and make sure we get the reply
	eng, mocktunnel, mockendpoints := setup(false)
	defer mocktunnel.Close()
	defer eng.Close()
	for _, endpoint := range mockendpoints {
		defer endpoint.Close()
	}
	//// Send a message to one of the mock endpoints
	msgs := make([][]byte, 3)
	msgs[0] = []byte("0 zz12345678901234567890123456789012345678901234567890123456789012345678901234567890aa")
	msgs[1] = []byte("1 xx12345678901234567890123456789012345678901234567890123456789012345678901234567890aa")
	msgs[2] = []byte("2 yy12345678901234567890123456789012345678901234567890123456789012345678901234567890aa")
	sendaddr := make([]*net.UDPAddr, 3)
	var err error
	sendaddr[0], err = net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", LISTENPORT1))
	util.CheckError(err)
	sendaddr[1], err = net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", LISTENPORT2))
	util.CheckError(err)
	sendaddr[2], err = net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", LISTENPORT3))
	util.CheckError(err)
	msgsrecvd := make(chan *Packet, 50)
	for _, conn := range mockendpoints {
		go simulateRemoteClient(conn, msgsrecvd)
	}

	/// The mock remotetunnel is a listener - we need to write to it
	tunneltoaddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", config.TunnelRemoteAddr, config.TunnelFrom))
	tunnelconn, err := net.DialUDP("udp", nil, tunneltoaddr)
	util.CheckError(err)
	fmt.Println("Tunnel conn addr ", tunneltoaddr.String())

	tmsg := messages.NewTunnelMessage(2048)
	time.Sleep(1 * time.Second) /// aloow time for everything to get started
	/// Send the messages
	for i, msg := range msgs {
		fullmessage := tmsg.Write(msg, sendaddr[i])
		_, err := tunnelconn.Write(fullmessage)
		util.CheckError(err)
	}

	/// Now read the messages
	for range msgs {
		to := time.NewTimer(2 * time.Second)
		select {
		case recvdmsg := <-msgsrecvd:
			//the messages should be the same and the address should match the one for given index
			indxpart := strings.Split(string(recvdmsg.data), " ")[0]
			indx, err := strconv.ParseInt(indxpart, 10, 32)
			util.CheckError(err)
			assert.Equal(t, sendaddr[indx].String(), recvdmsg.addr.String(), "Address mismatch")
			assert.Equal(t, msgs[indx], recvdmsg.data, "Data mismatch")
		case <-to.C:
			t.Error("Timed out waiting for message")
			break
		}
	}

}
