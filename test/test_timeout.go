/*
 * @Author: Opynicus
 * @Date: 2023-02-13 17:17:02
 * @LastEditTime: 2023-02-13 17:36:03
 * @LastEditors: Opynicus
 * @Description:
 * @FilePath: \ToyRPC\test\test_timeout.go
 * 可以输入预定的版权声明、个性签名、空行等
 */
package test

import (
	"ToyRPC/client"
	server "ToyRPC/service"
	"context"
	"log"
	"net"
	"sync"
	"time"
)

func (c Calc) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Num1))
	*reply = args.Num1 + args.Num2
	return nil
}

func xMethod(xc *client.XClient, ctx context.Context, typ, serviceMethod string, args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		log.Printf("%s %s error: %v", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

func startNewServer(addr chan string) {
	var c Calc
	ser := server.NewServer()
	if err := ser.Register(&c); err != nil {
		log.Fatal("register error:", err)
	}
	// pick a free port
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	ser.Accept(l)
}

func call(addr1, addr2 string) {
	d := client.NewMultiServerDiscovery([]string{"tcp@" + addr1, "tcp@" + addr2})
	xc := client.NewXClient(d, client.RandomSelect, nil)
	defer func() { _ = xc.Close() }()
	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			xMethod(xc, context.Background(), "call", "Calc.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcast(addr1, addr2 string) {
	d := client.NewMultiServerDiscovery([]string{"tcp@" + addr1, "tcp@" + addr2})
	xc := client.NewXClient(d, client.RandomSelect, nil)
	defer func() { _ = xc.Close() }()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			xMethod(xc, context.Background(), "broadcast", "Calc.Sum", &Args{Num1: i, Num2: i * i})
			// expect 2 - 5 timeout
			ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			xMethod(xc, ctx, "broadcast", "Calc.Sleep", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func TestTimeout() {
	log.SetFlags(0)
	ch1 := make(chan string)
	ch2 := make(chan string)
	// start two servers
	go startNewServer(ch1)
	go startNewServer(ch2)

	addr1 := <-ch1
	addr2 := <-ch2

	time.Sleep(time.Second)
	call(addr1, addr2)
	broadcast(addr1, addr2)
}
