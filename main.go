package main

import (
	rudp "rudp2/pkg"
	"flag"
)

var IsServer bool

func init() {
	flag.BoolVar(&IsServer, "s", false, "wheather is server")
	flag.Parse()
	rudp.SetCorruptTick(100)    //设置超过n个tick连接丢失
	rudp.SetExpiredTick(1e2 * 60 * 5)    //设置发送的消息最大保留n个tick
	rudp.SetSendDelayTick(3)  //设置n个tick发送一次消息包
	rudp.SetMissingTime(2 * 1e7)    //设置n纳秒没有收到消息包就认为消息丢失，请求重发
}

func main() {
	if IsServer {
		rudp.RunAsServer()
	}else {
		rudp.RunAsClient()
	}
}