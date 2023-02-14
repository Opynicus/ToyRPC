/*
 * @Author: Opynicus
 * @Date: 2023-02-10 16:20:39
 * @LastEditTime: 2023-02-14 16:25:42
 * @LastEditors: Opynicus
 * @Description:
 * @FilePath: \ToyRPC\main.go
 * 可以输入预定的版权声明、个性签名、空行等
 */
package main

import (
	"ToyRPC/client"
	"ToyRPC/registry"
	server "ToyRPC/service"
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type Calc int

type Args struct{ Num1, Num2 int }

func (c Calc) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (c Calc) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Num1))
	*reply = args.Num1 + args.Num2
	return nil
}

func startRegistry(wg *sync.WaitGroup) {
	l, _ := net.Listen("tcp", ":9999")
	registry.HandleHTTP()
	wg.Done()
	_ = http.Serve(l, nil)
}

func startServer(registryAddr string, wg *sync.WaitGroup) {
	var xMethod Calc
	l, _ := net.Listen("tcp", ":0")
	server := server.NewServer()
	_ = server.Register(&xMethod)
	registry.Heartbeat(registryAddr, "tcp@"+l.Addr().String(), 0)
	wg.Done()
	server.Accept(l)
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

func call(registry string) {
	rd := client.NewRegistryDiscovery(registry, 0)
	xc := client.NewXClient(rd, client.RandomSelect, nil)
	defer xc.Close()
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

func broadcast(registry string) {
	rd := client.NewRegistryDiscovery(registry, 0)
	xc := client.NewXClient(rd, client.RandomSelect, nil)
	defer xc.Close()
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

func main() {
	log.SetFlags(0)
	registryAddr := "http://localhost:9999/_toyrpc_/registry"
	var wg sync.WaitGroup
	wg.Add(1)
	go startRegistry(&wg)
	wg.Wait()

	time.Sleep(time.Second)
	wg.Add(2)
	go startServer(registryAddr, &wg)
	go startServer(registryAddr, &wg)
	wg.Wait()

	time.Sleep(time.Second)
	call(registryAddr)
	broadcast(registryAddr)
}
