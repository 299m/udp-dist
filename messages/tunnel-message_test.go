package messages

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestBasicWriteAndRead(t *testing.T) {
	// Create a new tunnel message
	tm := NewTunnelMessage(1024)
	// Create a new UDP address
	addr, _ := net.ResolveUDPAddr("udp", ":1234")
	// create some random data
	data := []byte("12345678901234567890123456789012345678901234567890123456789012345678901234567890")
	// put the message onto the tunnel message
	fullmsg := tm.Write(data, addr)
	// read the message from the tunnel message
	msgdata, needmore, readdr, nextmsgoffset, err := tm.Read(fullmsg)
	fmt.Println("in addr", addr.IP)
	fmt.Println("out addr", readdr.IP)
	assert.Equal(t, false, needmore, "Need more isn't false")
	assert.Equal(t, nil, err, "Error isn't nil")
	assert.Equal(t, addr.String(), readdr.String(), "Address isn't the same")
	assert.Equal(t, data, msgdata, "Data isn't the same")
	assert.Equal(t, len(fullmsg), nextmsgoffset, "Next message offset isn't 0")
}

func TestPartialMessageData(t *testing.T) {
	// Create a new tunnel message
	tm := NewTunnelMessage(1024)
	// Create a new UDP address
	addr, _ := net.ResolveUDPAddr("udp", ":1234")
	// create some random data
	data := []byte("12345678901234567890123456789012345678901234567890123456789012345678901234567890")
	// put the message onto the tunnel message
	fullmsg := tm.Write(data, addr)
	// read the message from the tunnel message
	msgdata, needmore, readdr, nextmsgoffset, err := tm.Read(fullmsg[:getHeaderSize()+12])
	assert.Equal(t, true, needmore, "Need more isn't true")
	assert.Equal(t, nil, err, "Error isn't nil")

	/// Now give it the full message and make sure everything matches
	msgdata, needmore, readdr, nextmsgoffset, err = tm.Read(fullmsg)
	assert.Equal(t, false, needmore, "Need more isn't false")
	assert.Equal(t, nil, err, "Error isn't nil")
	assert.Equal(t, addr.String(), readdr.String(), "Address isn't the same")
	assert.Equal(t, data, msgdata, "Data isn't the same")
	assert.Equal(t, len(fullmsg), nextmsgoffset, "Next message offset mismatch")
}

func TestPartialMessageHeader(t *testing.T) {
	// Create a new tunnel message
	tm := NewTunnelMessage(1024)
	// Create a new UDP address
	addr, _ := net.ResolveUDPAddr("udp", ":1234")
	// create some random data
	data := []byte("12345678901234567890123456789012345678901234567890123456789012345678901234567890")
	// put the message onto the tunnel message
	fullmsg := tm.Write(data, addr)
	// read the message from the tunnel message
	msgdata, needmore, readdr, nextmsgoffset, err := tm.Read(fullmsg[:getHeaderSize()-1])
	assert.Equal(t, true, needmore, "Need more isn't true")
	assert.Equal(t, nil, err, "Error isn't nil")

	/// Now give it the full message and make sure everything matches
	msgdata, needmore, readdr, nextmsgoffset, err = tm.Read(fullmsg)
	assert.Equal(t, false, needmore, "Need more isn't false")
	assert.Equal(t, nil, err, "Error isn't nil")
	assert.Equal(t, addr.String(), readdr.String(), "Address isn't the same")
	assert.Equal(t, data, msgdata, "Data isn't the same")
	assert.Equal(t, len(fullmsg), nextmsgoffset, "Next message offset mismatch")

}

func TestMultipleMessages(t *testing.T) {
	// Create a new tunnel message
	tm := NewTunnelMessage(1024)
	testbuf := bytes.Buffer{}
	// Create a new UDP address
	addr, _ := net.ResolveUDPAddr("udp", ":1234")
	// create some random data
	data := []byte("12345678901234567890123456789012345678901234567890123456789012345678901234567890")
	// put the message onto the tunnel message
	fullmsg := tm.Write(data, addr)
	testbuf.Write(fullmsg)

	// Create a new UDP address
	addr2, _ := net.ResolveUDPAddr("udp", ":3214")
	// create some random data
	data2 := []byte("a12345678901234567890123456789012345678901234567890123456789012345678901234567890z")
	// put the message onto the tunnel message
	fullmsg2 := tm.Write(data2, addr2)
	testbuf.Write(fullmsg2)

	// read the message from the tunnel message
	msgdata, needmore, readdr, nextmsgoffset, err := tm.Read(testbuf.Bytes())
	assert.Equal(t, false, needmore, "Need more isn't false")
	assert.Equal(t, nil, err, "Error isn't nil")
	assert.Equal(t, addr.String(), readdr.String(), "Address isn't the same")
	assert.Equal(t, string(data), string(msgdata), "Data isn't the same 1")

	msgdata, needmore, readdr, nextmsgoffset, err = tm.Read(testbuf.Bytes()[nextmsgoffset:])
	assert.Equal(t, false, needmore, "Need more isn't false")
	assert.Equal(t, nil, err, "Error isn't nil")
	assert.Equal(t, addr2.String(), readdr.String(), "Address isn't the same")
	assert.Equal(t, string(data2), string(msgdata), "Data isn't the same 2")
}
