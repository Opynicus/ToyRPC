/*
 * @Author: Opynicus
 * @Date: 2023-02-10 16:16:58
 * @LastEditTime: 2023-02-10 17:13:28
 * @LastEditors: Opynicus
 * @Description:
 * @FilePath: \ToyRPC\test\test_client.go
 * 可以输入预定的版权声明、个性签名、空行等
 */
package test

import (
	"ToyRPC/client"
	"ToyRPC/utils"
	"fmt"
	"log"
	"sync"
	"time"
)

const client_cnt = 10

func TestClient() {
	utils := utils.Utils{}
	addr := make(chan string)
	go utils.StartServer(addr)

	// in fact, following code is like a simple server client
	client, _ := client.Dial("tcp", <-addr)
	// defer func() { _ = connect.Close() }()
	defer client.Close()

	time.Sleep(time.Second)
	// send options
	var wg sync.WaitGroup
	for i := 0; i < client_cnt; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("ToyRPC req %d", i)
			var reply string
			if err := client.Call("RPC", args, &reply); err != nil {
				log.Fatal("call ToyRPC error:", err)
			}
			log.Println("reply:", reply)
		}(i)
	}
	wg.Wait()
}
