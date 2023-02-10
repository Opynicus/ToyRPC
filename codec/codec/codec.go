package codec

import "io"

const (
	GobType  string = "application/gob"
	JsonType string = "application/json" // not implemented
)

// Header is the header of a message.
type Header struct {
	ServiceMethod string
	Seq           uint64
	Error         string
}

// NewCodecFunc is a function that creates a new Codec.
type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

type NewCodecFunc func(io.ReadWriteCloser) Codec

// newCodecFuncMap is a map from a content type to a newCodecFunc.
var NewCodecFuncMap map[string]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[string]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}
