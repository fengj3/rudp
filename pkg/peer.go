package rudp

import (
	"errors"
)

type PeerMap struct {
	peers map[string]*RudpConn
	num int
}

func NewPeerMap() *PeerMap {
	return &PeerMap{peers: make(map[string]*RudpConn),}
}

func (pm *PeerMap) Add(rc *RudpConn) error {
	if rc.uid == "" {
		return errors.New("this rudpConn.uid is empty")
	}
	_, err := pm.Find(rc.uid)
	if err == nil {
		pm.peers[rc.uid] = rc
		return errors.New("this peer is exists")
	}
	pm.peers[rc.uid] = rc
	pm.num++
	return nil
}

func (pm *PeerMap) Find(uid string) (*RudpConn, error) {
	for iuid, irc := range pm.peers {
		if iuid == uid {
			return irc, nil
		}
	}
	return nil, errors.New("no found the peer")
}

func (pm *PeerMap) Delete(uid string) error {
	_, err := pm.Find(uid)
	if err != nil {
		return errors.New("this peer not is exists")
	}
	delete(pm.peers, uid)
	pm.num--
	return nil
}