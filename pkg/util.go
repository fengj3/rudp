package rudp

import (
	"time"
	"fmt"
	"log"
	"net"
)

// timeDate
// 返回时间
func timeDate() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// timestampNano
// 返回时间戳，毫秒??纳秒
func timestampNano() uint64 {
	return uint64(time.Now().UnixNano() / 1e6)
}

func dbg(format string, v ...interface{}) {
	if debug {
		log.Printf(format, v...)
	}
}

func checkErr(err error) {
	if err != nil {
		log.Printf("%v", err)
	}
}

func bitShow(n int) string {
	var ext string = "b"
	if n >= 1024 {
		n /= 1024
		ext = "Kb"
	}
	if n >= 1024 {
		n /= 1024
		ext = "Mb"
	}
	return fmt.Sprintf("%v %v", n, ext)
}

func AddrToUDPAddr(addr *net.Addr) *net.UDPAddr {
	udpAddr, _ := net.ResolveUDPAddr("udp", (*addr).String())
	return udpAddr
}