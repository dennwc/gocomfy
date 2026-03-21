package wsconn

func init() {
	RegisterEvent[*StatusEvent]()
}

type StatusEvent struct {
	SID    string `json:"sid,omitempty"`
	Status Status `json:"status"`
}

func (*StatusEvent) EventType() string {
	return "status"
}

type Status struct {
	Exec ExecInfo `json:"exec_info"`
}

type ExecInfo struct {
	Queue *int `json:"queue_remaining"`
}
