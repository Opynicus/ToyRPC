package client

import (
	"ToyRPC/codec"
	server "ToyRPC/service"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
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
	connect, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return newClient(connect, opt)
}

func (client *Client) Call(serviceMethod string, args interface{}, reply interface{}) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Err
}
