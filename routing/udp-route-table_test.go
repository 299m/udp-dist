package routing

import (
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestUdpRoutes_AddRoute(t *testing.T) {
	routes := NewUdpRoutes()
	netaddr := &net.UDPAddr{
		Port: 2345,
	}
	routes.AddRoute(netaddr, 24)
	found := routes.FindRouteById(24)
	assert.Equal(t, netaddr, found, "Route not found")
}

func TestUdpRoutes_FindRouteById(t *testing.T) {
	routes := NewUdpRoutes()
	netaddr := &net.UDPAddr{
		Port: 2345,
	}
	id := routes.FindOrAddRouteByAddr(netaddr)
	found := routes.FindRouteById(id)
	assert.Equal(t, netaddr, found, "Route not found")
}

func TestUdpRoutes_RemoveRouteById(t *testing.T) {
	routes := NewUdpRoutes()
	netaddr := &net.UDPAddr{
		Port: 2345,
	}
	routes.AddRoute(netaddr, 24)
	found := routes.FindRouteById(24)
	assert.Equal(t, netaddr, found, "Route not found")
	routes.RemoveRouteById(24)
	found = routes.FindRouteById(24)
	assert.Nil(t, found, "Route not removed")
}
