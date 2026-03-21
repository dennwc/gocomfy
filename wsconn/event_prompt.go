package wsconn

import (
	"encoding/json"

	"github.com/dennwc/gocomfy/types"
)

func init() {
	RegisterEvent[*ExecStart]()
	RegisterEvent[*ExecCached]()
	RegisterEvent[*ExecNode]()
	RegisterEvent[*ExecNodeDone]()
	RegisterEvent[*ExecSuccess]()
	RegisterEvent[*Progress]()
	RegisterEvent[*ProgressState]()
}

type PromptEvent interface {
	Event
	GetPromptID() string
}

type PromptEventBase struct {
	PromptID string `json:"prompt_id"`
}

func (e *PromptEventBase) GetPromptID() string {
	return e.PromptID
}

type ExecStart struct {
	PromptEventBase
	Time int64 `json:"timestamp"`
}

func (*ExecStart) EventType() string {
	return "execution_start"
}

type ExecCached struct {
	PromptEventBase
	Time  int64    `json:"timestamp"`
	Nodes []NodeID `json:"nodes"`
}

func (*ExecCached) EventType() string {
	return "execution_cached"
}

type ExecNode struct {
	PromptEventBase
	Node        *NodeID `json:"node"`
	DisplayNode *NodeID `json:"display_node,omitempty"`
}

func (*ExecNode) EventType() string {
	return "executing"
}

type NodeOutput struct {
	Images []types.ImageRef `json:"images"`
}

type ExecNodeDone struct {
	PromptEventBase
	Node        NodeID     `json:"node"`
	DisplayNode *NodeID    `json:"display_node,omitempty"`
	Output      NodeOutput `json:"output"`
}

func (*ExecNodeDone) EventType() string {
	return "executed"
}

type ExecSuccess struct {
	PromptEventBase
	Time int64 `json:"timestamp"`
}

func (*ExecSuccess) EventType() string {
	return "execution_success"
}

type ExecError struct {
	PromptEventBase
	Time           int64           `json:"timestamp"`
	Node           NodeID          `json:"node"`
	NodeType       string          `json:"node_type"`
	Executed       []NodeID        `json:"executed"`
	Exception      string          `json:"exception_message"`
	ExceptionType  string          `json:"exception_type"`
	Traceback      []string        `json:"traceback"`
	CurrentInputs  json.RawMessage `json:"current_inputs"`
	CurrentOutputs []NodeID        `json:"current_outputs"`
}

func (*ExecError) EventType() string {
	return "execution_error"
}

type Progress struct {
	PromptEventBase
	Node  NodeID  `json:"node"`
	Value float32 `json:"value"`
	Max   float32 `json:"max"`
}

func (*Progress) EventType() string {
	return "progress"
}

type ProgressState struct {
	PromptEventBase
	Nodes map[NodeID]*NodeProgress `json:"nodes"`
}

func (*ProgressState) EventType() string {
	return "progress_state"
}

type NodeState string

const (
	StateFinished NodeState = "finished"
	StateRunning  NodeState = "running"
)

type NodeProgress struct {
	PromptEventBase
	Node        NodeID    `json:"node_id"`
	RealNode    NodeID    `json:"real_node_id"`
	DisplayNode NodeID    `json:"display_node_id"`
	ParentNode  *NodeID   `json:"parent_node_id"`
	State       NodeState `json:"state"`
	Value       float32   `json:"value"`
	Max         float32   `json:"max"`
}
