package types

import (
	"fmt"
	"net/url"
)

type ImageType string

const (
	ImageInput  = ImageType("input")
	ImageTemp   = ImageType("temp")
	ImageOutput = ImageType("output")
)

type ImageRef struct {
	Filename  string    `json:"filename"`
	Subfolder string    `json:"subfolder"`
	Type      ImageType `json:"type"`
	Preview   string    `json:"preview,omitempty"`
}

func (r ImageRef) SetURL(vals url.Values) {
	vals.Set("filename", r.Filename)
	vals.Set("subfolder", r.Subfolder)
	vals.Set("type", string(r.Type))
	if r.Preview != "" {
		vals.Set("preview", r.Preview)
	}
}

// AnnotatedPath returns annotated path that is accepted by ComfyUI image load nodes.
func (r ImageRef) AnnotatedPath() string {
	if r.Subfolder != "" {
		return fmt.Sprintf("%s/%s [%s]", r.Subfolder, r.Filename, r.Type)
	}
	return fmt.Sprintf("%s [%s]", r.Filename, r.Type)
}
