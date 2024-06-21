package gocomfy

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/dennwc/gocomfy/graph/apigraph"
	"github.com/dennwc/gocomfy/graph/types"
)

func (c *Client) cancelAllPrompts(ctx context.Context) error {
	return c.postJSON(ctx, "/queue", struct {
		Clear bool `json:"clear"`
	}{
		Clear: true,
	}, nil)
}

func (c *Client) CancelPrompts(ctx context.Context, pids ...string) error {
	if len(pids) == 0 {
		return c.postJSON(ctx, "/queue", struct {
			Clear bool `json:"clear"`
		}{
			Clear: true,
		}, nil)
	}
	return c.postJSON(ctx, "/queue", struct {
		Delete []string `json:"delete"`
	}{
		Delete: pids,
	}, nil)
}

type PromptOption interface {
	todo()
}

func (c *Client) startPrompt(ctx context.Context, prompt any) (string, error) {
	var res struct {
		PromptID string `json:"prompt_id"`
	}
	err := c.postJSON(ctx, "/prompt", struct {
		ClientID string `json:"client_id"`
		Prompt   any    `json:"prompt"`
	}{
		ClientID: c.id,
		Prompt:   prompt,
	}, &res)
	if err != nil {
		return "", err
	}
	return res.PromptID, nil
}

func (c *Client) StartPromptJSON(ctx context.Context, raw json.RawMessage) (string, error) {
	return c.startPrompt(ctx, raw)
}

func (c *Client) StartPrompt(ctx context.Context, g *apigraph.Graph) (string, error) {
	return c.startPrompt(ctx, g)
}

type NodeResult struct {
	Images []ImageRef
}

type Results map[types.NodeID]NodeResult

func (c *Client) PromptResults(ctx context.Context, pid string) (Results, error) {
	var res map[string]struct {
		Outputs map[types.NodeID]struct {
			Images []ImageRef `json:"images"`
		}
	}
	err := c.getJSON(ctx, "/history/"+pid, &res)
	if err != nil {
		return nil, err
	}
	out := make(Results)
	for node, v := range res[pid].Outputs {
		out[node] = NodeResult{Images: v.Images}
	}
	return out, nil
}

func (c *Client) promptRaw(ctx context.Context, prompt any, opts ...PromptOption) (*Prompt, error) {
	if c.conn == nil {
		return nil, errors.New("websocket is not initialized")
	}
	c.mu.RLock()
	closed := c.prompts == nil
	c.mu.RUnlock()
	if closed {
		return nil, errors.New("connection closed")
	}
	pid, err := c.startPrompt(ctx, prompt)
	if err != nil {
		return nil, err
	}
	p := &Prompt{
		c:      c,
		log:    c.log.With("promptID", pid),
		pid:    pid,
		ctx:    ctx,
		events: make(chan Event, 10),
	}
	c.mu.Lock()
	if c.prompts == nil {
		c.mu.Unlock()
		return nil, errors.New("connection closed")
	}
	c.prompts[p.pid] = p
	c.mu.Unlock()
	return p, nil
}

func (c *Client) PromptJSON(ctx context.Context, prompt json.RawMessage, opts ...PromptOption) (*Prompt, error) {
	return c.promptRaw(ctx, prompt, opts...)
}

func (c *Client) Prompt(ctx context.Context, prompt *apigraph.Graph, opts ...PromptOption) (*Prompt, error) {
	return c.promptRaw(ctx, prompt, opts...)
}

func (c *Client) runPrompt(ctx context.Context, prompt any) (Results, error) {
	p, err := c.promptRaw(ctx, prompt)
	if err != nil {
		return nil, err
	}
	defer p.Close()

	// TODO: disable events for this one?
	for range p.Events() {
	}

	return p.Results(ctx)
}

func (c *Client) RunPromptJSON(ctx context.Context, prompt json.RawMessage) (Results, error) {
	return c.runPrompt(ctx, prompt)
}

func (c *Client) RunPrompt(ctx context.Context, prompt *apigraph.Graph) (Results, error) {
	return c.runPrompt(ctx, prompt)
}

func (c *Client) procPromptEvent(log *slog.Logger, pid string, typ string, raw json.RawMessage) {
	p := c.getPrompt(pid)
	if p == nil {
		log.Error("cannot find prompt")
		return
	}
	if err := p.processEvent(typ, raw); err != nil {
		p.log.Error("cannot process event", "err", err)
	}
}

type Prompt struct {
	c       *Client
	log     *slog.Logger
	pid     string
	ctx     context.Context
	events  chan Event
	curNode NodeID
	closed  atomic.Bool
}

func (p *Prompt) ID() string {
	return p.pid
}

func (p *Prompt) Events() <-chan Event {
	return p.events
}

func (p *Prompt) kill() bool {
	if !p.closed.CompareAndSwap(false, true) {
		return false // already closed
	}
	close(p.events)
	return true
}

func (p *Prompt) close() bool {
	if !p.kill() {
		return false // already closed
	}
	p.c.delPrompt(p.pid)
	return true
}

func (p *Prompt) Cancel(ctx context.Context) error {
	return p.c.CancelPrompts(ctx, p.pid)
}

func (p *Prompt) Close() {
	if !p.close() {
		return // already closed
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = p.Cancel(ctx)
	}()
}

func (p *Prompt) Results(ctx context.Context) (Results, error) {
	return p.c.PromptResults(ctx, p.pid)
}

func (p *Prompt) processEvent(typ string, raw json.RawMessage) error {
	if p.closed.Load() {
		return nil
	}
	switch typ {
	case "execution_start":
		return p.procExecStart(raw)
	case "execution_cached":
		return p.procExecCached(raw)
	case "executing":
		return p.procExecuting(raw)
	case "progress":
		return p.procProgress(raw)
	case "executed":
		return p.procExecuted(raw)
	default:
		p.log.Debug("unknown event")
		return nil
	}
}

func (p *Prompt) event(e Event) error {
	if p.closed.Load() {
		return nil
	}
	select {
	case <-p.ctx.Done():
		p.Close()
		return p.ctx.Err()
	case p.events <- e:
		return nil
	}
}

func (p *Prompt) procExecStart(raw json.RawMessage) error {
	return p.event(ExecStart{})
}

func (p *Prompt) procExecCached(raw json.RawMessage) error {
	var ev struct {
		Nodes []string `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		return err
	}
	var e ExecCache
	for _, node := range ev.Nodes {
		var id NodeID
		if err := id.Parse(node); err != nil {
			return err
		}
		e.Nodes = append(e.Nodes, id)
	}
	return p.event(e)
}

func (p *Prompt) curDone() error {
	if p.curNode == 0 {
		return nil
	}
	err := p.event(NodeDone{Node: p.curNode})
	p.curNode = 0
	return err
}

func (p *Prompt) procExecuting(raw json.RawMessage) error {
	var ev struct {
		Node *string `json:"node"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		return err
	}
	if err := p.curDone(); err != nil {
		return err
	}
	if ev.Node == nil {
		err := p.event(ExecDone{})
		p.close()
		return err
	}
	var id NodeID
	if err := id.Parse(*ev.Node); err != nil {
		return err
	}
	p.curNode = id
	return p.event(NodeStart{Node: id})
}

func (p *Prompt) procProgress(raw json.RawMessage) error {
	var ev struct {
		Node  string `json:"node"`
		Value int    `json:"value"`
		Max   int    `json:"max"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		return err
	}
	var id NodeID
	if err := id.Parse(ev.Node); err != nil {
		return err
	}
	return p.event(NodeProg{
		Node:  id,
		Value: ev.Value,
		Max:   ev.Max,
	})
}

func (p *Prompt) procExecuted(raw json.RawMessage) error {
	var ev struct {
		Node   string `json:"node"`
		Output struct {
			Images []ImageRef `json:"images"`
		} `json:"output"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		return err
	}
	var id NodeID
	if err := id.Parse(ev.Node); err != nil {
		return err
	}
	if p.curNode == id {
		p.curNode = 0
	}
	return p.event(NodeDone{
		Node: id,
		NodeResult: NodeResult{
			Images: ev.Output.Images,
		},
	})
}

type Event interface {
	isEvent()
}

type ExecStart struct{}

func (ExecStart) isEvent() {}

type ExecDone struct{}

func (ExecDone) isEvent() {}

type ExecCache struct {
	Nodes []NodeID
}

func (ExecCache) isEvent() {}

type NodeStart struct {
	Node NodeID
}

func (NodeStart) isEvent() {}

type NodeProg struct {
	Node  NodeID
	Value int
	Max   int
}

func (NodeProg) isEvent() {}

type NodeDone struct {
	Node NodeID
	NodeResult
}

func (NodeDone) isEvent() {}
