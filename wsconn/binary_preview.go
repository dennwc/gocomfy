package wsconn

import (
	"encoding/binary"
	"io"
)

func init() {
	RegisterBinary(BinaryPreviewImage, decodePreviewImage)
}

func decodePreviewImage(r io.Reader) (BinaryEvent, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return nil, err
	}
	typ := PreviewType(binary.BigEndian.Uint32(buf[:4]))
	return &PreviewImage{
		Type:   typ,
		Reader: r,
	}, nil
}

type PreviewType int32

const (
	PreviewJPG = PreviewType(1)
	PreviewPNG = PreviewType(2)
)

type PreviewImage struct {
	Type   PreviewType
	Reader io.Reader
}

func (*PreviewImage) EventType() BinaryType {
	return BinaryPreviewImage
}

func (b *PreviewImage) WriteTo(w io.Writer) (int64, error) {
	var buf [8]byte
	binary.BigEndian.PutUint32(buf[0:4], uint32(b.EventType()))
	binary.BigEndian.PutUint32(buf[4:8], uint32(b.Type))
	hsz, err := w.Write(buf[:8])
	if err != nil {
		return int64(hsz), err
	}
	n, err := io.Copy(w, b.Reader)
	return n + int64(hsz), err
}
