package utils

import (
	"ToyRPC/codec/server"
	"log"
	"net"
)

type Utils struct {
	rpc_cnt int
}

func (u *Utils) StartServer(addr chan string) {
	// pick a free port
	link, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", link.Addr())
	addr <- link.Addr().String()
	server.Accept(link)
}
