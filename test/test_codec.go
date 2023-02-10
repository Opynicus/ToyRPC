/*
 * @Author: Opynicus
 * @Date: 2023-02-10 10:06:44
 * @LastEditTime: 2023-02-10 16:58:25
 * @LastEditors: Opynicus
 * @Description:
 * @FilePath: \ToyRPC\test\test_codec.go
 * 可以输入预定的版权声明、个性签名、空行等
 */
package test

import (
	"ToyRPC/codec/codec"
	"ToyRPC/codec/server"
	"ToyRPC/utils"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

const rpc_cnt = 10

func TestCodec() {
	utils := utils.Utils{}
	addr := make(chan string)
	go utils.StartServer(addr)

	// in fact, following code is like a simple server client
	connect, _ := net.Dial("tcp", <-addr)
	// defer func() { _ = connect.Close() }()
	defer connect.Close()

	time.Sleep(time.Second)
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
