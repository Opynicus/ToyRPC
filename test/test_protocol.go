/*
 * @Author: Opynicus
 * @Date: 2023-02-12 20:09:53
 * @LastEditTime: 2023-02-12 20:14:33
 * @LastEditors: Opynicus
 * @Description:
 * @FilePath: \ToyRPC\test\test_protocol.go
 * 可以输入预定的版权声明、个性签名、空行等
 */
package test

import (
	"ToyRPC/client"
	server "ToyRPC/service"
	"log"
	"net"
	"os"
	"runtime"
)

func TestProtocol() {
	if runtime.GOOS == "linux" {
		ch := make(chan struct{})
		addr := "/tmp/geerpc.sock"
		go func() {
			_ = os.Remove(addr)
			l, err := net.Listen("unix", addr)
			if err != nil {
				log.Print("failed to listen unix socket")
			}
			ch <- struct{}{}
			server.Accept(l)
		}()
		<-ch
		client.XDial("unix@" + addr)
	}
}
