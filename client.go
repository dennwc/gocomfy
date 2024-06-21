package gocomfy

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type clientOptions struct {
	Log         *slog.Logger
	NoWebSocket bool
	WSDialer    *websocket.Dialer
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

func WithWSDialer(dialer *websocket.Dialer) ClientOption {
	return clientOptionFunc(func(c *clientOptions) {
		c.WSDialer = dialer
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
	if opt.WSDialer == nil {
		opt.WSDialer = websocket.DefaultDialer
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
	var conn *websocket.Conn
	if !opt.NoWebSocket {
		wsurl := fmt.Sprintf("ws://%s/ws?clientId=%s", host, sid)
		conn, _, err = opt.WSDialer.DialContext(ctx, wsurl, nil)
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
	conn    *websocket.Conn
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
		typ, r, err := c.conn.NextReader()
		if errors.Is(err, net.ErrClosed) {
			return
		} else if err != nil {
			c.log.Error("websocket error", "err", err, "type", fmt.Sprintf("%T", err))
			return
		}
		switch typ {
		case websocket.CloseMessage:
			return
		case websocket.TextMessage:
			c.procTextEvent(r)
		case websocket.BinaryMessage:
			var buf [64]byte
			n1, _ := r.Read(buf[:])
			n2, _ := io.Copy(io.Discard, r)
			data := buf[:n1]
			c.log.Info("websocket binary data", "size", n1+int(n2), "data", hex.EncodeToString(data))
		}
	}
}

func (c *Client) procTextEvent(r io.Reader) {
	var ev struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(r).Decode(&ev); err != nil {
		c.log.Error("cannot decode websocket event", "err", err)
		return
	}
	log := c.log.With("type", ev.Type)
	var pe struct {
		PromptID string `json:"prompt_id"`
	}
	if err := json.Unmarshal(ev.Data, &pe); err == nil && pe.PromptID != "" {
		log = log.With("promptID", pe.PromptID)
	}
	log.Debug("websocket event", "data", ev.Data)
	if pe.PromptID != "" {
		c.procPromptEvent(log, pe.PromptID, ev.Type, ev.Data)
	} else {
		c.procClientEvent(log, ev.Type, ev.Data)
	}
}

func (c *Client) procClientEvent(log *slog.Logger, typ string, raw json.RawMessage) {
	switch typ {
	case "status":
		c.procStatus(log, raw)
	default:
		log.Debug("unknown client event")
	}
}

func (c *Client) procStatus(log *slog.Logger, raw json.RawMessage) {
	if c.onQueueSize == nil {
		return
	}
	var ev struct {
		Status struct {
			Exec struct {
				Queue *int `json:"queue_remaining"`
			} `json:"exec_info"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		log.Error("cannot decode status event", "err", err)
		return
	}
	if q := ev.Status.Exec.Queue; q != nil {
		c.onQueueSize(*q)
	}
}

func (c *Client) get(ctx context.Context, path string) (io.ReadCloser, error) {
	addr := fmt.Sprintf("http://%s%s", c.host, path)
	req, err := http.NewRequestWithContext(ctx, "GET", addr, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.hcli.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	rc, err := c.get(ctx, path)
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
