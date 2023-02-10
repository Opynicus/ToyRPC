/*
 * @Author: Opynicus
 * @Date: 2023-02-09 14:32:55
 * @LastEditTime: 2023-02-10 10:07:00
 * @LastEditors: Opynicus
 * @Description:
 * @FilePath: \ToyRPC\test\test_codec.go
 * 可以输入预定的版权声明、个性签名、空行等
 */
package main

import (
	"ToyRPC/codec/codec"
	"ToyRPC/codec/server"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

const rpc_cnt = 10

func startServer(addr chan string) {
	// pick a free port
	link, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", link.Addr())
	addr <- link.Addr().String()
	server.Accept(link)
}

func main() {
	addr := make(chan string)
	go startServer(addr)

	// in fact, following code is like a simple server client
	connect, _ := net.Dial("tcp", <-addr)
	// defer func() { _ = connect.Close() }()
	defer connect.Close()

	time.Sleep(time.Second)
	// send options
	json.NewEncoder(connect).Encode(server.DefaultOption)
	cc := codec.NewGobCodec(connect)
	// send request & receive response
	for i := 0; i < rpc_cnt; i++ {
		header := &codec.Header{
			ServiceMethod: "Arith",
			Seq:           uint64(i),
		}
		cc.Write(header, fmt.Sprintf("server req %d", header.Seq))
		cc.ReadHeader(header)
		var reply string
		cc.ReadBody(&reply)
		log.Println("reply:", reply)
	}
}
