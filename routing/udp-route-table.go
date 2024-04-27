package routing

import (
	"net"
	"sync"
	"sync/atomic"
)

type UdpRoutes struct {
	routes       map[int64]*net.UDPAddr
	ids          map[net.Addr]int64
	associations map[int64]int64
	assoclock    sync.RWMutex

	rwlock sync.RWMutex
	nextid int64
}

func NewUdpRoutes() *UdpRoutes {
	return &UdpRoutes{
		routes:       make(map[int64]*net.UDPAddr),
		ids:          make(map[net.Addr]int64),
		associations: make(map[int64]int64),
	}
}

func (u *UdpRoutes) Associate(localid, remoteid int64) {
	u.assoclock.Lock()
	defer u.assoclock.Unlock()

	u.associations[localid] = remoteid
	u.associations[remoteid] = localid
}

func (u *UdpRoutes) FindAssociation(id int64) (int64, bool) {
	u.assoclock.RLock()
	defer u.assoclock.RUnlock()

	if assoc, ok := u.associations[id]; ok {
		return assoc, true
	}
	return 0, false
}

func (u *UdpRoutes) AddRoute(from *net.UDPAddr, id int64) {
	u.rwlock.Lock()
	defer u.rwlock.Unlock()
	//// Update to the new route
	u.routes[id] = from
	u.ids[from] = id
}

func (u *UdpRoutes) FindRouteById(id int64) *net.UDPAddr {
	u.rwlock.RLock()
	defer u.rwlock.RUnlock()

	if route, ok := u.routes[id]; ok {
		return route
	}
	return nil
}

func (u *UdpRoutes) RemoveRouteById(id int64) {
	u.rwlock.Lock()
	defer u.rwlock.Unlock()

	delete(u.routes, id)
}

func (u *UdpRoutes) FindRouteByAddr(addr *net.UDPAddr) (int64, bool) {
	u.rwlock.Lock()
	defer u.rwlock.Unlock()

	if id, ok := u.ids[addr]; ok {
		return id, true
	}
	return 0, false
}

func (u *UdpRoutes) FindOrAddRouteByAddr(addr *net.UDPAddr) int64 {
	id, found := u.FindRouteByAddr(addr)
	if found {
		return id
	}
	////Else add a new route
	id = atomic.AddInt64(&u.nextid, 1)
	u.AddRoute(addr, id)
	return id
}
