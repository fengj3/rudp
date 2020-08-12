package rudp

import (
	"time"
)

var pingPacket []byte = []byte{0x00}

var (
	MAX_PACKET_SIZE int = 1000
	MAX_HEADER_SIZE int = 2 + 6 + 6
	MAX_DATA_SIZE int = MAX_PACKET_SIZE - MAX_HEADER_SIZE
)

//rudp
var corruptTick int = 5
var expiredTick int = 1e2 * 60 * 5 //5 minute on sendTick 1e7
var sendDelayTick int = 1
var missingTime int = 1e7

func SetCorruptTick(tick int)   { corruptTick = tick }
func SetExpiredTick(tick int)   { expiredTick = tick }
func SetSendDelayTick(tick int) { sendDelayTick = tick }
func SetMissingTime(miss int)   { missingTime = miss }

//rudp conn
var debug bool = false
var autoSend bool = true
var sendTick time.Duration = 1e7
var maxSendNumPerTick int = 500

func SetDebug(d bool)                { debug = d }
func SetAtuoSend(send bool)          { autoSend = send }
func SetSendTick(tick time.Duration) { sendTick = tick }
func SetMaxSendNumPerTick(n int)     { maxSendNumPerTick = n }