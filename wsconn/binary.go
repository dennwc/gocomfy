package wsconn

import (
	"encoding/binary"
	"fmt"
	"io"
)

// See https://github.com/Comfy-Org/ComfyUI/blob/master/protocol.py

type BinaryType int32

const (
	BinaryPreviewImage          = BinaryType(1)
	BinaryUnencodedPreviewImage = BinaryType(2)
	BinaryText                  = BinaryType(3)
	BinaryPreviewImageWithMeta  = BinaryType(4)
)

type BinaryDecodeFunc func(r io.Reader) (BinaryEvent, error)

func newBinaryRegistry[K comparable]() *binaryRegistry[K] {
	return &binaryRegistry[K]{
		byType: make(map[K]BinaryDecodeFunc),
	}
}

type binaryRegistry[K comparable] struct {
	byType map[K]BinaryDecodeFunc
}

func (r *binaryRegistry[K]) Register(typ K, fnc BinaryDecodeFunc) {
	if _, ok := r.byType[typ]; ok {
		panic(fmt.Errorf("already registered: %v", typ))
	}
	r.byType[typ] = fnc
}

func (r *binaryRegistry[K]) Get(typ K) BinaryDecodeFunc {
	return r.byType[typ]
}

var byTypeBin = newBinaryRegistry[BinaryType]()

func RegisterBinary(typ BinaryType, fnc BinaryDecodeFunc) {
	byTypeBin.Register(typ, fnc)
}

type BinaryEvent interface {
	EventType() BinaryType
	WriteTo(w io.Writer) (int64, error)
}

type RawBinaryEvent struct {
	Type   BinaryType
	Reader io.Reader // only valid once and before the next ReadMsg
}

func (e *RawBinaryEvent) EventType() BinaryType {
	return e.Type
}
func (e *RawBinaryEvent) Decode() (BinaryEvent, error) {
	fnc := byTypeBin.Get(e.Type)
	if fnc == nil {
		return e, nil
	}
	return fnc(e.Reader)
}
func (e *RawBinaryEvent) WriteTo(w io.Writer) (int64, error) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(e.Type))
	hsz, err := w.Write(buf[:])
	if err != nil {
		return int64(hsz), err
	}
	n, err := io.Copy(w, e.Reader)
	return n + int64(hsz), err
}
