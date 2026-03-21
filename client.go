package gocomfy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/dennwc/gocomfy/wsconn"
	"github.com/google/uuid"
)

type clientOptions struct {
	Log         *slog.Logger
	NoWebSocket bool
	WSOptions   []wsconn.DialOption
	HTTPClient  *http.Client
	OnQueueSize func(queue int)
}

type ClientOption interface {
	applyToClient(c *clientOptions)
}

type clientOptionFunc func(c *clientOptions)

func (f clientOptionFunc) applyToClient(c *clientOptions) {
	f(c)
}

func WithWSDialOptions(opts ...wsconn.DialOption) ClientOption {
	return clientOptionFunc(func(c *clientOptions) {
		c.WSOptions = append(c.WSOptions, opts...)
	})
}

func WithHTTPClient(cli *http.Client) ClientOption {
	return clientOptionFunc(func(c *clientOptions) {
		c.HTTPClient = cli
	})
}

func WithOnQueueSize(fnc func(queue int)) ClientOption {
	return clientOptionFunc(func(c *clientOptions) {
		c.OnQueueSize = fnc
	})
}

func WithoutWebsocket() ClientOption {
	return clientOptionFunc(func(c *clientOptions) {
		c.NoWebSocket = true
	})
}

func NewClient(ctx context.Context, host string, opts ...ClientOption) (*Client, error) {
	var opt clientOptions
	for _, o := range opts {
		o.applyToClient(&opt)
	}
	if opt.Log == nil {
		opt.Log = slog.Default()
	}
	if opt.HTTPClient == nil {
		opt.HTTPClient = http.DefaultClient
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	sid := id.String()
	opt.Log = opt.Log.With("clientId", sid)
	var conn *wsconn.Conn
	if !opt.NoWebSocket {
		conn, err = wsconn.Dial(ctx, host, sid, opt.WSOptions...)
		if err != nil {
			return nil, err
		}
	}
	c := &Client{
		id:          sid,
		host:        host,
		log:         opt.Log,
		conn:        conn,
		hcli:        opt.HTTPClient,
		prompts:     make(map[string]*Prompt),
		onQueueSize: opt.OnQueueSize,
	}
	if !opt.NoWebSocket {
		go c.readEvents()
	}
	return c, nil
}

type Client struct {
	id   string
	host string
	log  *slog.Logger
	hcli *http.Client

	onQueueSize func(queue int)

	mu      sync.RWMutex
	conn    *wsconn.Conn
	prompts map[string]*Prompt
}

func (c *Client) ID() string {
	return c.id
}

func (c *Client) killPrompts() {
	c.mu.Lock()
	prompts := c.prompts
	c.prompts = nil
	c.mu.Unlock()
	for _, p := range prompts {
		p.kill()
	}
}

func (c *Client) Close() {
	c.killPrompts()
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

func (c *Client) getPrompt(pid string) *Prompt {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.prompts[pid]
}

func (c *Client) delPrompt(pid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.prompts, pid)
}

func (c *Client) readEvents() {
	defer c.killPrompts()
	for {
		m, err := c.conn.ReadMsg()
		if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
			return
		} else if err != nil {
			c.log.Error("websocket error", "err", err, "type", fmt.Sprintf("%T", err))
			return
		}
		switch m := m.(type) {
		case *wsconn.EventMsg:
			c.procEvent(m.Event)
		}
	}
}

func (c *Client) procEvent(ev wsconn.Event) {
	log := c.log.With("type", ev.EventType())
	log.Debug("websocket event", "data", ev)
	if pe, ok := ev.(wsconn.PromptEvent); ok {
		c.procPromptEvent(log, pe)
	} else {
		c.procClientEvent(log, ev)
	}
}

func (c *Client) procClientEvent(log *slog.Logger, ev wsconn.Event) {
	switch ev := ev.(type) {
	case *wsconn.StatusEvent:
		c.procStatus(log, ev)
	default:
		log.Debug("unknown client event")
	}
}

func (c *Client) procStatus(log *slog.Logger, ev *wsconn.StatusEvent) {
	if c.onQueueSize == nil {
		return
	}
	if q := ev.Status.Exec.Queue; q != nil {
		c.onQueueSize(*q)
	}
}

func (c *Client) get(ctx context.Context, path string) (io.ReadCloser, string, error) {
	addr := fmt.Sprintf("http://%s%s", c.host, path)
	req, err := http.NewRequestWithContext(ctx, "GET", addr, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.hcli.Do(req)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return resp.Body, resp.Header.Get("Content-Type"), nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	rc, _, err := c.get(ctx, path)
	if err != nil {
		return err
	}
	defer rc.Close()
	return json.NewDecoder(rc).Decode(out)
}

func (c *Client) postJSON(ctx context.Context, path string, data, out any) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(data)
	if err != nil {
		return err
	}
	addr := fmt.Sprintf("http://%s%s", c.host, path)
	req, err := http.NewRequestWithContext(ctx, "POST", addr, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hcli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
