package wsconn

import (
	"encoding/binary"
	"io"
	"strings"
)

func init() {
	RegisterBinary(BinaryText, decodeText)
}

func decodeText(r io.Reader) (BinaryEvent, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return nil, err
	}
	idsz := binary.BigEndian.Uint32(buf[:])
	id := make([]byte, idsz)
	if _, err := io.ReadFull(r, id); err != nil {
		return nil, err
	}
	var sbuf strings.Builder
	_, err := io.Copy(&sbuf, r)
	if err != nil {
		return nil, err
	}
	return &Text{
		Node: NodeID(id),
		Text: sbuf.String(),
	}, nil
}

type Text struct {
	Node NodeID
	Text string
}

func (*Text) EventType() BinaryType {
	return BinaryText
}

func (b *Text) WriteTo(w io.Writer) (int64, error) {
	var buf [8]byte
	binary.BigEndian.PutUint32(buf[0:4], uint32(b.EventType()))
	binary.BigEndian.PutUint32(buf[4:8], uint32(len(b.Node)))
	hsz, err := w.Write(buf[:8])
	if err != nil {
		return int64(hsz), err
	}
	msz, err := w.Write([]byte(b.Node))
	if err != nil {
		return int64(hsz) + int64(msz), err
	}
	n, err := w.Write([]byte(b.Text))
	return int64(n + hsz + msz), err
}
