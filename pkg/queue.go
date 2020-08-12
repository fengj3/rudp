package rudp

import (
	"bytes"
)

type MessageQueue struct {
	head *MessageItem   //头部
	tail *MessageItem   //尾部
	num int   //总数
}

type MessageItem struct {
	byteBuffer bytes.Buffer   //消息体
	id int   //消息序号
	tick int   //逻辑时钟
	next *MessageItem   //下一个
}

func (mq *MessageQueue) pop(id int) *MessageItem {
	if mq.head == nil {
		return nil
	}
	m := mq.head
	if id >= 0 && m.id != id {
		return nil
	}
	mq.head = m.next
	m.next = nil
	if mq.head == nil {
		mq.tail = nil
	}
	mq.num--
	return m
}

func (mq *MessageQueue) push(nmi *MessageItem) {
	if mq.tail == nil {
		mq.head = nmi
		mq.tail = nmi
	}else {
		mq.tail.next = nmi
		mq.tail = nmi
	}
	mq.num++
}

func NewMessageItem(byteBuffer bytes.Buffer, id int, tick int) *MessageItem {
	return &MessageItem{
		byteBuffer: byteBuffer,
		id: id,
		tick: tick,
		next: nil,
	}
}