package gocomfy

import (
	"context"
	"io"

	"github.com/dennwc/gocomfy/graph/classes"
)

func (c *Client) ObjectsInfoRaw(ctx context.Context) (io.ReadCloser, error) {
	rc, _, err := c.get(ctx, "/object_info")
	return rc, err
}

func (c *Client) ObjectsInfo(ctx context.Context) (classes.Classes, error) {
	rc, err := c.ObjectsInfoRaw(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return classes.Decode(rc)
}
