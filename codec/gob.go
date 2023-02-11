/*
 * @Author: Opynicus
 * @Date: 2023-02-09 11:46:19
 * @LastEditTime: 2023-02-09 16:30:18
 * @LastEditors: Opynicus
 * @Description:
 * @FilePath: \ToyRPC\codec\gob.go
 * 可以输入预定的版权声明、个性签名、空行等
 */
package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

// GobCodec is a codec that uses gob to encode/decode.
type GobCodec struct {
	connect io.ReadWriteCloser
	buf     *bufio.Writer
	dec     *gob.Decoder
	enc     *gob.Encoder
}

var _ Codec = (*GobCodec)(nil) // ensure GobCodec implements codec

// NewGobCodec returns a new GobCodec.
func NewGobCodec(connect io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(connect)
	return &GobCodec{
		connect: connect,
		buf:     buf,
		dec:     gob.NewDecoder(connect),
		enc:     gob.NewEncoder(buf),
	}
}

// ReadHeader reads the header from the connection.
func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}

// ReadBody reads the body from the connection.
func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

// Write writes the header and body to the connection.
func (c *GobCodec) Write(h *Header, body interface{}) (err error) {
	// ensure the connection is closed if there is an error
	defer func() {
		// flush the buffer
		c.buf.Flush()
		if err != nil {
			c.Close()
		}
	}()
	// encode the header and body failed, close the connection
	if err = c.enc.Encode(h); err != nil {
		log.Println("rpc: gob error encoding header:", err)
		return
	}
	if err = c.enc.Encode(body); err != nil {
		log.Println("rpc: gob error encoding body:", err)
		return
	}
	return
}

// Close closes the connection.
func (c *GobCodec) Close() error {
	return c.connect.Close()
}
