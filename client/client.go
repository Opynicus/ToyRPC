package client

import (
	"ToyRPC/codec"
	server "ToyRPC/service"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	MagicNumber      = 0x3bef5c
	connected        = "200 Connected to ToyRPC"
	defaultRPCPath   = "/_toyrpc_" // default RPC path
	defaultDebugPath = "/debug/toyrpc"
)

// Client represents an RPC Client.
type Client struct {
	cc       codec.Codec      // for encode and decode request and response
	opt      *server.Option   // for codec header
	send_mtx sync.Mutex       // protect following
	header   codec.Header     // for request
	mtx      sync.Mutex       // protect following
	seq      uint64           // sequence number for requests
	pending  map[uint64]*Call // store calls that are waiting for server response
	closing  bool             // user has called Close
	shutdown bool             // server has told us to stop
}

type clientResult struct {
	client *Client
	err    error
}

func (client *Client) Close() error {
	client.mtx.Lock()
	defer client.mtx.Unlock()
	if client.closing {
		return errors.New("rpc client: already closed")
	}
	client.closing = true
	return client.cc.Close()
}

var _ io.Closer = (*Client)(nil)

func (client *Client) IsAvailable() bool {
	client.mtx.Lock()
	defer client.mtx.Unlock()
	return !client.shutdown && !client.closing
}

// register registers the call
func (client *Client) register(call *Call) (uint64, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()
	if client.closing || client.shutdown {
		return 0, errors.New("rpc client is closing")
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil
}

// remove removes the call from pending calls
func (client *Client) remove(seq uint64) *Call {
	client.mtx.Lock()
	defer client.mtx.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

// terminate terminates all pending calls
func (client *Client) terminate(err error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()
	for _, call := range client.pending {
		call.Err = err
		call.done()
	}
}

func (client *Client) send(call *Call) {
	client.send_mtx.Lock()
	defer client.send_mtx.Unlock()
	seq, err := client.register(call)
	if err != nil {
		call.Err = err
		call.done()
		return
	}
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""
	if err := client.cc.Write(&client.header, call.Args); err != nil {
		call := client.remove(seq)
		if call != nil {
			call.Err = err
			call.done()
		}
	}
}

func (client *Client) Go(serviceMethod string, args interface{}, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	client.send(call)
	return call
}

func (client *Client) receive() {
	var err error
	for err == nil {
		var header codec.Header
		// read header
		if err = client.cc.ReadHeader(&header); err != nil {
			break
		}
		call := client.remove(header.Seq)
		switch {
		case call == nil:
			// it usually means that Write partially failed
			// and call was already removed.
			err = client.cc.ReadBody(nil)
		case header.Error != "":
			call.Err = fmt.Errorf(header.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default:
			// normal case
			if err = client.cc.ReadBody(call.Reply); err != nil {
				call.Err = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	// error occurs, so terminateCalls pending calls
	client.terminate(err)
}

func newClientCodec(cc codec.Codec, opt *server.Option) *Client {
	client := &Client{
		seq:     1, // seq starts with 1, 0 means invalid call
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	go client.receive()
	return client
}

func newClient(connect net.Conn, opt *server.Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("rpc client: codec not found: %s", opt.CodecType)
		log.Println("rpc client: codec error:", err)
		return nil, err
	}
	if err := json.NewEncoder(connect).Encode(opt); err != nil {
		log.Println("rpc client: options error: ", err)
		_ = connect.Close()
		return nil, err
	}
	return newClientCodec(f(connect), opt), nil
}

func parseOptions(opts ...*server.Option) (*server.Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return server.DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}
	opt := opts[0]
	opt.MagicNumber = server.DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = server.DefaultOption.CodecType
	}
	return opt, nil
}

func Dial(network, address string, opts ...*server.Option) (*Client, error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	// connect, err := net.Dial(network, address)
	connect, err := net.DialTimeout(network, address, opt.ConnTimeOut)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = connect.Close()
		}
	}()
	// execute newClient in a goroutine
	ch := make(chan clientResult)
	go func() {
		c, err := newClient(connect, opt)
		ch <- clientResult{c, err}
	}()
	// no timeout
	if opt.ConnTimeOut == 0 {
		result := <-ch
		return result.client, result.err
	}

	select {
	case <-time.After(opt.ConnTimeOut): // time.After() means the time after the timeout, not the time before the timeout
		return nil, errors.New("rpc client: connect timeout: expect within " + opt.ConnTimeOut.String())
	case result := <-ch: // if the connection is established before the timeout, the result will be returned
		return result.client, result.err
	}
}

func (client *Client) CallWithoutTimeout(serviceMethod string, args interface{}, reply interface{}) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Err
}

func (client *Client) Call(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	call := client.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case <-ctx.Done(): // if the context is canceled, the call will be removed from the pending map
		client.remove(call.Seq)
		return errors.New("rpc client: call failed: " + ctx.Err().Error())
	case call := <-call.Done: // if the call is done, the result will be returned
		return call.Err
	}
}

func NewHTTPClient(conn net.Conn, opt *server.Option) (*Client, error) {
	io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", defaultRPCPath))
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err != nil && resp.Status == connected {
		return newClient(conn, opt)
	}
	if err != nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	return nil, err
}

func DialHTTP(network, address string, opts ...*server.Option) (*Client, error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	// connect, err := net.Dial(network, address)
	connect, err := net.DialTimeout(network, address, opt.ConnTimeOut)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = connect.Close()
		}
	}()
	// execute newClient in a goroutine
	ch := make(chan clientResult)
	go func() {
		c, err := NewHTTPClient(connect, opt)
		ch <- clientResult{c, err}
	}()
	// no timeout
	if opt.ConnTimeOut == 0 {
		result := <-ch
		return result.client, result.err
	}

	select {
	case <-time.After(opt.ConnTimeOut): // time.After() means the time after the timeout, not the time before the timeout
		return nil, errors.New("rpc client: connect timeout: expect within " + opt.ConnTimeOut.String())
	case result := <-ch: // if the connection is established before the timeout, the result will be returned
		return result.client, result.err
	}
}

// XDial calls different functions to connect to a RPC server
// according the first parameter rpcAddr.
// rpcAddr is a general format (protocol@addr) to represent a rpc server
// eg, http@10.0.0.1:7001, tcp@10.0.0.1:9999, unix@/tmp/toyrpc.sock
func XDial(rpcAddr string, opts ...*server.Option) (*Client, error) {
	parts := strings.Split(rpcAddr, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("rpc client err: wrong format '%s', expect protocol@addr", rpcAddr)
	}
	protocol, addr := parts[0], parts[1]
	switch protocol {
	case "http":
		return DialHTTP("tcp", addr, opts...)
	default:
		// tcp, unix or other transport protocol
		return Dial(protocol, addr, opts...)
	}
}
