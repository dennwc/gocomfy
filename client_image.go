package gocomfy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
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
}

func (r ImageRef) setURL(vals url.Values) {
	vals.Set("filename", r.Filename)
	vals.Set("subfolder", r.Subfolder)
	vals.Set("type", string(r.Type))
}

// AnnotatedPath returns annotated path that is accepted by ComfyUI image load nodes.
func (r ImageRef) AnnotatedPath() string {
	if r.Subfolder != "" {
		return fmt.Sprintf("%s/%s [%s]", r.Subfolder, r.Filename, r.Type)
	}
	return fmt.Sprintf("%s [%s]", r.Filename, r.Type)
}

func (c *Client) GetImageFile(ctx context.Context, ref ImageRef) (io.ReadCloser, error) {
	vals := make(url.Values)
	ref.setURL(vals)
	return c.get(ctx, "/view?"+vals.Encode())
}

func (c *Client) GetImage(ctx context.Context, ref ImageRef) (image.Image, error) {
	rc, err := c.GetImageFile(ctx, ref)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return png.Decode(rc)
}

func (c *Client) UploadImageFile(ctx context.Context, ref ImageRef, r io.Reader, overwrite bool) (*ImageRef, error) {
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	if ref.Type != "" {
		form.WriteField("type", string(ref.Type))
	}
	if ref.Subfolder != "" {
		form.WriteField("subfolder", ref.Subfolder)
	}
	if overwrite {
		form.WriteField("overwrite", "true")
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Type", "image/png")
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename=%q`, ref.Filename))
	fw, err := form.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(fw, r); err != nil {
		return nil, err
	}
	if err = form.Close(); err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("http://%s/upload/image", c.host)
	req, err := http.NewRequestWithContext(ctx, "POST", addr, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := c.hcli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var out struct {
		Filename  string    `json:"name"`
		Subfolder string    `json:"subfolder"`
		Type      ImageType `json:"type"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &ImageRef{
		Filename:  out.Filename,
		Subfolder: out.Subfolder,
		Type:      out.Type,
	}, nil
}

func (c *Client) UploadImage(ctx context.Context, ref ImageRef, img image.Image, overwrite bool) (*ImageRef, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return c.UploadImageFile(ctx, ref, &buf, overwrite)
}
