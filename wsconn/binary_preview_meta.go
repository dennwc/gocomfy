package wsconn

import (
	"encoding/binary"
	"encoding/json"
	"io"
)

func init() {
	RegisterBinary(BinaryPreviewImageWithMeta, decodePreviewImageMeta)
}

func decodePreviewImageMeta(r io.Reader) (BinaryEvent, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return nil, err
	}
	hsz := binary.BigEndian.Uint32(buf[:])
	meta := make([]byte, hsz)
	if _, err := io.ReadFull(r, meta); err != nil {
		return nil, err
	}
	return &PreviewImageMeta{
		Meta:   meta,
		Reader: r,
	}, nil
}

type PreviewMeta struct {
	ImageType string `json:"image_type"`
}

type PreviewImageMeta struct {
	Meta   json.RawMessage
	Reader io.Reader
}

func (*PreviewImageMeta) EventType() BinaryType {
	return BinaryPreviewImageWithMeta
}

func (b *PreviewImageMeta) WriteTo(w io.Writer) (int64, error) {
	var buf [8]byte
	binary.BigEndian.PutUint32(buf[0:4], uint32(b.EventType()))
	binary.BigEndian.PutUint32(buf[4:8], uint32(len(b.Meta)))
	hsz, err := w.Write(buf[:8])
	if err != nil {
		return int64(hsz), err
	}
	msz, err := w.Write(b.Meta)
	if err != nil {
		return int64(hsz + msz), err
	}
	n, err := io.Copy(w, b.Reader)
	return n + int64(hsz+msz), err
}
