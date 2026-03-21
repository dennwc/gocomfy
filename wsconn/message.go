package wsconn

type Message interface {
	isMessage()
}

type BinaryMsg struct {
	Event BinaryEvent
}

func (*BinaryMsg) isMessage() {}

type EventMsg struct {
	Event Event
}

func (*EventMsg) isMessage() {}
