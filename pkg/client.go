package rudp

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	"bytes"
)

var (
	serverUDPAddr *net.UDPAddr
	clientUDPAddr *net.UDPAddr
	pc *net.PacketConn
	pm *PeerMap
)

func init() {
	serverUDPAddr, _ = net.ResolveUDPAddr("udp4", "172.245.40.9:51222")
	clientUDPAddr, _ = net.ResolveUDPAddr("udp4", "198.46.158.230:51222")
}

func read(rc *RudpConn) {
	for {
		buffer := make([]byte, MAX_PACKET_SIZE)
		n, err := rc.Read(buffer)
		if err != nil {
			fmt.Printf("ReadFrom err %v\n", err)
			break
		}
		if n == 1 && bytes.Equal(buffer[:n], pingPacket) {
			continue
		}
		fmt.Printf("receive ")
		for i := range buffer[:n] {
			v := int(buffer[i])
			fmt.Printf("%d", v)
		}
		fmt.Printf(" from <%v>\n", rc.remoteAddr.String())
	}
}

func receiveFromClient(rl *RudpListener) {
	for {
		rconn, err := rl.AcceptRudp()
		if err != nil {
			fmt.Printf("accept err %v\n", err)
			break
		}
		go read(rconn)
	}
}

func RunAsClient() {
	pm = NewPeerMap()
	
	nPc, err := net.ListenPacket("udp", ":51222")
	if err != nil {
		fmt.Println("runAsClient", err)
		os.Exit(-3)
	}
	
	pc = &nPc
	
	rc := NewConn(serverUDPAddr, NewRudp())
	rc.setUid("manager")
	pm.Add(rc)
	
	listener := NewListener()
	
	defer func() { fmt.Println("defer close", nPc.Close()) }()
	
	go func() {
		for i := uint8(0); ; i++ {
			_, err = rc.Write([]byte{byte(i), byte(i), byte(i)})
			if err != nil {
				fmt.Printf("rc.send err %v\n", err)
				os.Exit(1)
				break
			}
			// time.Sleep(1*time.Second)
		}
	}()
	
	go heartBeat()
	go receiveFromClient(listener)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT)
	select {
	case <-signalChan:
	}
}

func heartBeat() {
	for {
		_, err := (*pc).WriteTo(pingPacket, serverUDPAddr)
		if err != nil {
			fmt.Println("one client exit by an error occurred while ping.")
		}
		time.Sleep(5*1000)
	}
}