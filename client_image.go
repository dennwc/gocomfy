package gocomfy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"iter"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"

	"github.com/dennwc/gocomfy/types"
)

type ImageType = types.ImageType

const (
	ImageInput  = types.ImageInput
	ImageTemp   = types.ImageTemp
	ImageOutput = types.ImageOutput
)

type ImageRef = types.ImageRef

func (c *Client) GetImageFile(ctx context.Context, ref ImageRef) (io.ReadCloser, string, error) {
	vals := make(url.Values)
	ref.SetURL(vals)
	return c.get(ctx, "/view?"+vals.Encode())
}

func (c *Client) GetImage(ctx context.Context, ref ImageRef) (image.Image, error) {
	rc, typ, err := c.GetImageFile(ctx, ref)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	switch typ {
	case "image/png":
		return png.Decode(rc)
	case "image/jpeg":
		return jpeg.Decode(rc)
	}
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

type ListAssetsOpts struct {
	Offset int
	Limit  int
}

type Asset struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Hash       string   `json:"asset_hash"`
	Size       int64    `json:"size"`
	Mime       int64    `json:"mime_type"`
	Tags       []string `json:"tags"`
	UserMeta   string   `json:"user_metadata"`
	PreviewID  string   `json:"preview_id"`
	PreviewURL string   `json:"preview_url"`
	PromptID   string   `json:"prompt_id"`
}

func (c *Client) ListAssetsPage(ctx context.Context, opts *ListAssetsOpts) ([]Asset, error) {
	if opts == nil {
		opts = &ListAssetsOpts{}
	}
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	qu := make(url.Values)
	qu.Set("offset", strconv.Itoa(opts.Offset))
	qu.Set("limit", strconv.Itoa(opts.Limit))

	var out struct {
		Items []Asset `json:"assets"`
	}
	err := c.getJSON(ctx, "/api/assets?"+qu.Encode(), out)
	if err != nil {
		return nil, err
	}
	return out.Items, nil
}

func (c *Client) ListAssetsSeq(ctx context.Context, opts *ListAssetsOpts) iter.Seq2[Asset, error] {
	return func(yield func(Asset, error) bool) {
		if opts == nil {
			opts = &ListAssetsOpts{}
		}
		for {
			list, err := c.ListAssetsPage(ctx, opts)
			if err != nil {
				yield(Asset{}, err)
				return
			}
			if len(list) == 0 {
				return
			}
			opts.Offset += len(list)
			for _, a := range list {
				if !yield(a, nil) {
					return
				}
			}
		}
	}
}
