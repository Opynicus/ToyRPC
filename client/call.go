package client

type Call struct {
	Seq           uint64
	ServiceMethod string
	Args          interface{}
	Reply         interface{}
	Err           error
	Done          chan *Call
}

func (call *Call) done() {
	call.Done <- call
}
