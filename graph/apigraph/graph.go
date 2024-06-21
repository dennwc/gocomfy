package apigraph

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/dennwc/gocomfy/graph/types"
)

func ReadFile(path string) (*Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Read(f)
}

func WriteFile(path string, g *Graph) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = Write(f, g); err != nil {
		return err
	}
	return f.Close()
}

func Write(w io.Writer, g *Graph) error {
	return json.NewEncoder(w).Encode(g)
}

func Read(r io.Reader) (*Graph, error) {
	var jg jsonGraph
	if err := json.NewDecoder(r).Decode(&jg); err != nil {
		return nil, err
	}
	return fromJSON(jg)
}

func Marshal(g *Graph) ([]byte, error) {
	return json.Marshal(g)
}

func Unmarshal(data []byte) (*Graph, error) {
	var jg jsonGraph
	if err := json.Unmarshal(data, &jg); err != nil {
		return nil, err
	}
	return fromJSON(jg)
}

type jsonGraph map[types.NodeID]jsonNode

type jsonNode struct {
	Class  types.NodeClass            `json:"class_type"`
	Inputs map[string]json.RawMessage `json:"inputs,omitempty"`
	Meta   json.RawMessage            `json:"_meta,omitempty"`
}

type Meta struct {
	Title string `json:"title,omitempty"`
}

func valueFromJSON(raw json.RawMessage) (Value, error) {
	var s *string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == nil {
			return nil, nil
		}
		return String(*s), nil
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		if v, err := n.Int64(); err == nil {
			return Int(v), nil
		}
		v, err := n.Float64()
		if err != nil {
			return nil, err
		}
		return Float(v), nil
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return Bool(b), nil
	}
	var l Link
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, err
	}
	return l, nil
}

func fromJSON(jg jsonGraph) (*Graph, error) {
	g := &Graph{Nodes: make(map[types.NodeID]*Node, len(jg))}
	for id, jn := range jg {
		n := &Node{
			ID:     id,
			Class:  jn.Class,
			Inputs: make(map[string]Value, len(jn.Inputs)),
			Meta:   jn.Meta,
		}
		for name, raw := range jn.Inputs {
			v, err := valueFromJSON(raw)
			if err != nil {
				return nil, fmt.Errorf("cannot parse input %v.%s: %w", id, name, err)
			}
			n.Inputs[name] = v
		}
		g.Nodes[id] = n
		g.LastID = max(g.LastID, id)
	}
	return g, nil
}

var _ json.Marshaler = Graph{}

func New() *Graph {
	return &Graph{Nodes: make(map[types.NodeID]*Node)}
}

type Graph struct {
	LastID types.NodeID
	Nodes  map[types.NodeID]*Node
}

func (g Graph) MarshalJSON() ([]byte, error) {
	return json.Marshal(g.Nodes)
}

func (g *Graph) Add(n *Node) types.NodeID {
	g.LastID++
	id := g.LastID
	n.ID = id
	g.Nodes[id] = n
	return id
}

type Value interface {
	isValue()
}

type Int int64

func (Int) isValue() {}

type Float float64

func (Float) isValue() {}

type String string

func (String) isValue() {}

type Bool bool

func (Bool) isValue() {}

var (
	_ json.Unmarshaler = (*Link)(nil)
	_ json.Marshaler   = Link{}
)

type Link struct {
	NodeID  types.NodeID
	OutPort int
}

func (l *Link) UnmarshalJSON(data []byte) error {
	var (
		sid  string
		port int
	)
	arr := [2]any{&sid, &port}
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	id, err := strconv.ParseUint(sid, 10, 64)
	if err != nil {
		return err
	}
	*l = Link{
		NodeID:  types.NodeID(id),
		OutPort: port,
	}
	return nil
}

func (l Link) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{l.NodeID.String(), l.OutPort})
}

func (Link) isValue() {}

type Node struct {
	ID     types.NodeID     `json:"-"`
	Class  types.NodeClass  `json:"class_type"`
	Inputs map[string]Value `json:"inputs,omitempty"`
	Meta   json.RawMessage  `json:"_meta,omitempty"`
}
