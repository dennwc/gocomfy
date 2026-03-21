package wsconn

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/gorilla/websocket"
)

type DialOption func(opts *dialOptions)

type dialOptions struct {
	WSDialer *websocket.Dialer
}

func WithDialer(dialer *websocket.Dialer) DialOption {
	return func(opts *dialOptions) {
		opts.WSDialer = dialer
	}
}

func Dial(ctx context.Context, host string, clientID string, opts ...DialOption) (*Conn, error) {
	return DialURL(ctx, fmt.Sprintf("ws://%s/ws", host), clientID, opts...)
}

func DialURL(ctx context.Context, addr string, clientID string, opts ...DialOption) (*Conn, error) {
	var opt dialOptions
	for _, o := range opts {
		o(&opt)
	}
	if opt.WSDialer == nil {
		opt.WSDialer = websocket.DefaultDialer
	}
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	if q == nil {
		q = make(url.Values)
	}
	if q.Get("clientId") == "" {
		q.Set("clientId", clientID)
	}
	u.RawQuery = q.Encode()
	c, _, err := opt.WSDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return nil, err
	}
	return NewConn(c), nil
}

func NewConn(c *websocket.Conn) *Conn {
	return &Conn{c: c}
}

type Conn struct {
	c *websocket.Conn
}

func (c *Conn) Close() error {
	return c.c.Close()
}

func (c *Conn) writeEvent(e any) error {
	wc, err := c.c.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	defer wc.Close()
	return json.NewEncoder(wc).Encode(e)
}

func (c *Conn) WriteEvent(e Event) error {
	if r, ok := e.(*RawEvent); ok {
		return c.writeEvent(r)
	}
	return c.writeEvent(eventMsg[Event]{
		Type: e.EventType(),
		Data: e,
	})
}

func (c *Conn) ReadMsgRaw() (Message, error) {
	for {
		typ, r, err := c.c.NextReader()
		if err != nil {
			return nil, err
		}
		switch typ {
		case websocket.CloseMessage:
			return nil, io.EOF
		case websocket.TextMessage:
			var e RawEvent
			err := json.NewDecoder(r).Decode(&e)
			if err != nil {
				return nil, err
			}
			return &EventMsg{&e}, nil
		case websocket.BinaryMessage:
			var hdr [4]byte
			_, err := io.ReadFull(r, hdr[:])
			if err != nil {
				return nil, err
			}
			e := &RawBinaryEvent{
				Type:   BinaryType(binary.BigEndian.Uint32(hdr[:4])),
				Reader: r,
			}
			return &BinaryMsg{e}, nil
		}
	}
}

func (c *Conn) ReadMsg() (Message, error) {
	m, err := c.ReadMsgRaw()
	if err != nil {
		return nil, err
	}
	switch m := m.(type) {
	case *EventMsg:
		if r, ok := m.Event.(*RawEvent); ok {
			ev, err := r.Decode()
			if err != nil {
				return nil, err
			}
			m.Event = ev
		}
	case *BinaryMsg:
		if r, ok := m.Event.(*RawBinaryEvent); ok {
			ev, err := r.Decode()
			if err != nil {
				return nil, err
			}
			m.Event = ev
		}
	}
	return m, nil
}
