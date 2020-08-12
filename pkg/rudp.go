package rudp

import(
	"bytes"
	"errors"
	"sync/atomic"
	"time"
)

const (
	TYPE_PING = iota
	TYPE_EOF
	TYPE_CORRUPT
	TYPE_REQUEST
	TYPE_MISSING
	TYPE_NORMAL
)

const (
	MAX_MSG_HEAD    = 4
	GENERAL_PACKAGE = 576 - 60 - 8
	MAX_PACKAGE     = 0x7fff - TYPE_NORMAL
)

const (
	ERROR_NIL int32 = iota
	ERROR_EOF
	ERROR_REMOTE_EOF
	ERROR_CORRUPT
	ERROR_MSG_SIZE
)

type Error struct {
	v int32
}

func (e *Error) Load() int32   { return atomic.LoadInt32(&e.v) }
func (e *Error) Store(n int32) { atomic.StoreInt32(&e.v, n) }

func (e *Error) Error() error {
	switch e.Load() {
	case ERROR_EOF:
		return errors.New("EOF")
	case ERROR_REMOTE_EOF:
		return errors.New("remote EOF")
	case ERROR_CORRUPT:
		return errors.New("corrupt")
	case ERROR_MSG_SIZE:
		return errors.New("recive msg size error")
	default:
		return nil
	}
}

type Package struct {
	Next *Package
	Bts  []byte
}

type PackageBuffer struct {
	tmp  bytes.Buffer
	num  int
	head *Package
	tail *Package
}

//打包
func (tmp *PackageBuffer) packRequest(min, max, tag int) {
	if tmp.tmp.Len() > MAX_DATA_SIZE {
		tmp.newPackage()
	}
	tmp.tmp.WriteByte(byte(tag))//包类型
	tmp.tmp.WriteByte(byte((min & 0xff00) >> 8))//min高8位
	tmp.tmp.WriteByte(byte(min & 0xff))//min低8位
	tmp.tmp.WriteByte(byte((max & 0xff00) >> 8))//max高8位
	tmp.tmp.WriteByte(byte(max & 0xff))//max低8位
}

//填充头部
func (tmp *PackageBuffer) fillHeader(head, id int) {
	if head < 128 {
		tmp.tmp.WriteByte(byte(head))//bts length
	} else {
		//bts length >= 128
		tmp.tmp.WriteByte(byte(((head & 0x7f00) >> 8) | 0x80))//bts length 取高7位(去除最高位)
		tmp.tmp.WriteByte(byte(head & 0xff))//bts length 取低8位,防止>255
	}
	tmp.tmp.WriteByte(byte((id & 0xff00) >> 8))//msg id 高8位
	tmp.tmp.WriteByte(byte(id & 0xff))//msg id 低8位
}

func (tmp *PackageBuffer) packMessage(m *MessageItem) {
	if m.byteBuffer.Len()+tmp.tmp.Len() >= MAX_DATA_SIZE {
		tmp.newPackage()
	}
	tmp.fillHeader(m.byteBuffer.Len()+TYPE_NORMAL, m.id)
	tmp.tmp.Write(m.byteBuffer.Bytes())
}

func (tmp *PackageBuffer) newPackage() {
	if tmp.tmp.Len() <= 0 {
		return
	}
	p := &Package{Bts: make([]byte, tmp.tmp.Len())}
	copy(p.Bts, tmp.tmp.Bytes())
	tmp.tmp.Reset()
	tmp.num++
	if tmp.tail == nil {
		tmp.head = p
		tmp.tail = p
	} else {
		tmp.tail.Next = p
		tmp.tail = p
	}
}

type Rudp struct {
	recvQueue    MessageQueue
	recvSkip     map[int]int
	reqSendAgain chan [2]int
	recvIDMin    int
	recvIDMax    int

	sendQueue    MessageQueue
	sendHistory  MessageQueue
	addSendAgain chan [2]int
	sendID       int

	corrupt      Error

	currentTick       int
	lastRecvTick      int
	lastExpiredTick   int
	lastSendDelayTick int
}

func NewRudp() *Rudp {
	return &Rudp{reqSendAgain: make(chan [2]int, 1<<10), addSendAgain: make(chan [2]int, 1<<10), recvSkip: make(map[int]int)}
}

func (r *Rudp) Recv(bts []byte) (int, error) {
	if err := r.corrupt.Load(); err != ERROR_NIL {
		return 0, r.corrupt.Error()
	}
	m := r.recvQueue.pop(r.recvIDMin)
	if m == nil {
		return 0, nil
	}
	r.recvIDMin++
	copy(bts, m.byteBuffer.Bytes())
	return m.byteBuffer.Len(), nil
}

func (r *Rudp) send(bts []byte) (int, error) {
	if err := r.corrupt.Load(); err != ERROR_NIL {
		return 0, r.corrupt.Error()
	}
	if len(bts) > MAX_DATA_SIZE {
		return 0, errors.New("byte array size more then MAX_DATA_SIZE")
	}
	m := &MessageItem{}
	m.byteBuffer.Write(bts)
	m.id = r.sendID
	r.sendID++
	m.tick = r.currentTick
	r.sendQueue.push(m)
	return len(bts), nil
}

//update tick
func (r *Rudp) Update(tick int) *Package {
	if r.corrupt.Load() != ERROR_NIL {
		return nil
	}
	r.currentTick += tick
	if r.currentTick >= r.lastExpiredTick+expiredTick {
		r.lastExpiredTick = r.currentTick
		r.clearSendExpired()
	}
	if r.currentTick >= r.lastRecvTick+corruptTick {
		r.corrupt.Store(ERROR_CORRUPT)
	}
	if r.currentTick >= r.lastSendDelayTick+sendDelayTick {
		r.lastSendDelayTick = r.currentTick
		return r.outPut()
	}
	return nil
}

func (r *Rudp) getID(max int, bt1, bt2 byte) int {
	n1, n2 := int(bt1), int(bt2)
	id := n1*256 + n2
	id |= max & ^0xffff//id = id | 0x00
	if id < max-0x8000 {
		id += 0x10000
		dbg("id < max-0x8000 ,net %v,id %v,min %v,max %v,cur %v",
			n1*256+n2, id, r.recvIDMin, max, id+0x10000)
	} else if id > max+0x8000 {
		id -= 0x10000
		dbg("id > max-0x8000 ,net %v,id %v,min %v,max %v,cur %v",
			n1*256+n2, id, r.recvIDMin, max, id+0x10000)
	}
	return id
}

func (r *Rudp) outPut() *Package {
	var tmp PackageBuffer
	r.reqMissing(&tmp)
	r.replyRequest(&tmp)
	r.sendMessage(&tmp)
	if tmp.head == nil && tmp.tmp.Len() == 0 {
		tmp.tmp.WriteByte(byte(TYPE_PING))
	}
	tmp.newPackage()
	return tmp.head
}

func (r *Rudp) Input(bts []byte) {
	sz := len(bts)
	if sz > 0 {
		r.lastRecvTick = r.currentTick
	}
	for sz > 0 {
		len := int(bts[0])
		if len > 127 {
			if sz <= 1 {
				r.corrupt.Store(ERROR_MSG_SIZE)
				return
			}
			len = (len*256 + int(bts[1])) & 0x7fff
			bts = bts[2:]
			sz -= 2
		} else {
			bts = bts[1:]
			sz -= 1
		}
		switch len {
		case TYPE_PING:
			r.checkMissing(false)
		case TYPE_EOF:
			r.corrupt.Store(ERROR_EOF)
		case TYPE_CORRUPT:
			r.corrupt.Store(ERROR_REMOTE_EOF)
			return
		case TYPE_REQUEST, TYPE_MISSING:
			if sz < 4 {
				r.corrupt.Store(ERROR_MSG_SIZE)
				return
			}
			exe := r.addRequest
			max := r.sendID
			if len == TYPE_MISSING {
				exe = r.addMissing
				max = r.recvIDMax
			}
			exe(r.getID(max, bts[0], bts[1]), r.getID(max, bts[2], bts[3]))
			bts = bts[4:]
			sz -= 4
		default:
			len -= TYPE_NORMAL
			if sz < len+2 {
				r.corrupt.Store(ERROR_MSG_SIZE)
				return
			}
			r.insertMessage(r.getID(r.recvIDMax, bts[0], bts[1]), bts[2:len+2])
			bts = bts[len+2:]
			sz -= len + 2
		}
	}
	r.checkMissing(false)
}

func (r *Rudp) checkMissing(direct bool) {
	head := r.recvQueue.head
	if head != nil && head.id > r.recvIDMin {
		nano := int(time.Now().UnixNano())
		last := r.recvSkip[r.recvIDMin]
		if !direct && last == 0 {
			r.recvSkip[r.recvIDMin] = nano
			dbg("miss start %v-%v,max %v", r.recvIDMin, head.id-1, r.recvIDMax)
		} else if direct || last+missingTime < nano {
			delete(r.recvSkip, r.recvIDMin)
			r.reqSendAgain <- [2]int{r.recvIDMin, head.id - 1}
			dbg("req miss %v-%v,direct %v,wait num %v",
				r.recvIDMin, head.id-1, direct, r.recvQueue.num)
		}
	}
}

func (r *Rudp) insertMessage(id int, bts []byte) {
	if id < r.recvIDMin {
		dbg("already recv %v,len %v", id, len(bts))
		return
	}
	delete(r.recvSkip, id)
	if id > r.recvIDMax || r.recvQueue.head == nil {
		m := &MessageItem{}
		m.byteBuffer.Write(bts)
		m.id = id
		r.recvQueue.push(m)
		r.recvIDMax = id
	} else {
		m := r.recvQueue.head
		last := &r.recvQueue.head
		for m != nil {
			if m.id == id {
				dbg("repeat recv id %v,len %v", id, len(bts))
			} else if m.id > id {
				tmp := &MessageItem{}
				tmp.byteBuffer.Write(bts)
				tmp.id = id
				tmp.next = m
				*last = tmp
				r.recvQueue.num++
				return
			}
			last = &m.next
			m = m.next
		}
	}
}

func (r *Rudp) sendMessage(tmp *PackageBuffer) {
	m := r.sendQueue.head
	for m != nil {
		tmp.packMessage(m)
		m = m.next
	}
	if r.sendQueue.head != nil {
		if r.sendHistory.tail == nil {
			r.sendHistory = r.sendQueue
		} else {
			r.sendHistory.tail.next = r.sendQueue.head
			r.sendHistory.tail = r.sendQueue.tail
		}
		r.sendQueue.head = nil
		r.sendQueue.tail = nil
	}
}
func (r *Rudp) clearSendExpired() {
	m := r.sendHistory.head
	for m != nil {
		if m.tick >= r.lastExpiredTick {
			break
		}
		m = m.next
	}
	r.sendHistory.head = m
	if m == nil {
		r.sendHistory.tail = nil
	}
}

func (r *Rudp) addRequest(min, max int) {
	dbg("add request %v-%v,max send id %v", min, max, r.sendID)
	r.addSendAgain <- [2]int{min, max}
}

func (r *Rudp) addMissing(min, max int) {
	if max < r.recvIDMin {
		dbg("add missing %v-%v fail,already recv,min %v", min, max, r.recvIDMin)
		return
	}
	if min > r.recvIDMin {
		dbg("add missing %v-%v fail, more than min %v", min, max, r.recvIDMin)
		return
	}
	head := 0
	if r.recvQueue.head != nil {
		head = r.recvQueue.head.id
	}
	dbg("add missing %v-%v,min %v,head %v", min, max, r.recvIDMin, head)
	r.recvIDMin = max + 1
	r.checkMissing(true)
}

func (r *Rudp) replyRequest(tmp *PackageBuffer) {
	for {
		select {
		case again := <-r.addSendAgain:
			history := r.sendHistory.head
			min, max := again[0], again[1]
			if history == nil || max < history.id {
				dbg("send again miss %v-%v,send max %v", min, max, r.sendID)
				tmp.packRequest(min, max, TYPE_MISSING)
			} else {
				var start, end, num int
				for {
					if history == nil || max < history.id {
						//expired
						break
					} else if min <= history.id {
						tmp.packMessage(history)
						if start == 0 {
							start = history.id
						}
						end = history.id
						num++
					}
					history = history.next
				}
				if min < start {
					tmp.packRequest(min, start-1, TYPE_MISSING)
					dbg("send again miss %v-%v,send max %v", min, start-1, r.sendID)
				}
				dbg("send again %v-%v of %v-%v,all %v,max send id %v", start, end, min, max, num, r.sendID)
			}
		default:
			return
		}
	}
}

func (r *Rudp) reqMissing(tmp *PackageBuffer) {
	for {
		select {
		case req := <-r.reqSendAgain:
			tmp.packRequest(req[0], req[1], TYPE_REQUEST)
		default:
			return
		}
	}
}