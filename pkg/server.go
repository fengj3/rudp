package rudp

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func receiveFromServer(rc *RudpConn) {
	for {
		buffer := make([]byte, MAX_PACKET_SIZE)
		n, err := rc.Read(buffer)
		if err != nil {
			fmt.Printf("ReadFrom err %v\n", err)
			break
		}
		fmt.Printf("receive ")
		for i := range buffer[:n] {
			v := int(buffer[i])
			fmt.Printf("%d", v)
		}
		fmt.Printf("packet-received: bytes=%d from=%s\n", n, rc.remoteAddr.String())
	}
}

func RunAsServer() {
	pm = NewPeerMap()
	
	nPc, err := net.ListenPacket("udp", ":51222")
	if err != nil {
		fmt.Println("runAsServer", err)
		os.Exit(-3)
	}
	
	pc = &nPc
	
	rc := NewConn(clientUDPAddr, NewRudp())
	rc.setUid("client")
	pm.Add(rc)
	
	listener := NewListener()
	
	defer func() { fmt.Println("defer close", nPc.Close()) }()
	
	go func() {
		for {
			rconn, err := listener.AcceptRudp()
			if err != nil {
				fmt.Printf("accept err %v\n", err)
				break
			}
			go receiveFromServer(rconn)
		}
	}()
	
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT)
	select {
	case <-signalChan:
	}
}