package rudp

import (
	"net"
	"time"
)

func NewConn(remoteAddr *net.UDPAddr, rudp *Rudp) *RudpConn {
	con := &RudpConn{conn: pc, rudp: rudp,
		recvChan: make(chan []byte, 1<<16), recvErr: make(chan error, 2),
		sendChan: make(chan []byte, 1<<16), sendErr: make(chan error, 2),
		SendTick: make(chan int, 2), remoteAddr: remoteAddr,
		in: make(chan []byte, 1<<16),
	}
	go con.run()
	return con
}

type RudpConn struct {
	conn *net.PacketConn

	rudp *Rudp

	recvChan chan []byte
	recvErr  chan error

	sendChan chan []byte
	sendErr  chan error

	SendTick chan int

	uid string
	remoteAddr *net.UDPAddr
	in  chan []byte
}

func (rc *RudpConn) setUid(uid string) {
	rc.uid = uid
}

func (rc *RudpConn) LocalAddr() net.Addr                { return (*rc.conn).LocalAddr() }
func (rc *RudpConn) Connected() bool                    { return rc.remoteAddr == nil }

func (rc *RudpConn) RemoteAddr() net.Addr {
	return rc.remoteAddr
}

func (rc *RudpConn) Close() error {
	_, err := (*rc.conn).WriteTo([]byte{TYPE_CORRUPT}, rc.remoteAddr)
	rc.in <- []byte{TYPE_EOF}
	checkErr(err)
	return err
}

func (rc *RudpConn) Read(bts []byte) (n int, err error) {
	select {
	case data := <-rc.recvChan:
		copy(bts, data)
		return len(data), nil
	case err := <-rc.recvErr:
		return 0, err
	}
}

func (rc *RudpConn) send(bts []byte) (err error) {
	select {
	case rc.sendChan <- bts:
		return nil
	case err := <-rc.sendErr:
		return err
	}
}

func (rc *RudpConn) Write(bts []byte) (n int, err error) {
	sz := len(bts)
	for len(bts)+MAX_MSG_HEAD > GENERAL_PACKAGE {
		if err := rc.send(bts[:GENERAL_PACKAGE-MAX_MSG_HEAD]); err != nil {
			return 0, err
		}
		bts = bts[GENERAL_PACKAGE-MAX_MSG_HEAD:]
	}
	return sz, rc.send(bts)
}

func (rc *RudpConn) rudpRecv(data []byte) error {
	for {
		n, err := rc.rudp.Recv(data)
		if err != nil {
			rc.recvErr <- err
			return err
		} else if n == 0 {
			break
		}
		bts := make([]byte, n)
		copy(bts, data[:n])
		rc.recvChan <- bts
	}
	return nil
}

func (rc *RudpConn) recvLoop() {
	data := make([]byte, MAX_PACKET_SIZE)
	for {
		select {
		case bts := <- rc.in:
			rc.rudp.Input(bts)
			if rc.rudpRecv(data) != nil {
				return
			}
		}
	}
}

func (rc *RudpConn) sendLoop() {
	var sendNum int
	for {
		select {
		case tick := <-rc.SendTick:
		sendOut:
			for {
				select {
				case bts := <-rc.sendChan:
					_, err := rc.rudp.send(bts)
					if err != nil {
						rc.sendErr <- err
						return
					}
					sendNum++
					if sendNum >= maxSendNumPerTick {
						break sendOut
					}
				default:
					break sendOut
				}
			}
			sendNum = 0
			p := rc.rudp.Update(tick)
			var num, sz int
			for p != nil {
				n, err := int(0), error(nil)
				n, err = (*rc.conn).WriteTo(p.Bts, rc.remoteAddr)
				if err != nil {
					rc.sendErr <- err
					return
				}
				sz, num = sz+n, num+1
				p = p.Next
			}
			if num > 1 {
				show := bitShow(sz * int(time.Second/sendTick))
				dbg("send package num %v,sz %v, %v/s,local %v,remote %v",
					num, show, show, rc.LocalAddr(), rc.RemoteAddr())
			}
		}
	}
}

func (rc *RudpConn) run() {
	if autoSend && sendTick > 0 {
		go func() {
			tick := time.Tick(sendTick)
			for {
				select {
				case <-tick:
					rc.SendTick <- 1
				}
			}
		}()
	}
	go rc.recvLoop()
	rc.sendLoop()
}