package wsconn

import (
	"encoding/json"
	"reflect"
)

var byType = make(map[string]reflect.Type)

func RegisterEvent[T Event]() {
	var zero T
	rt := reflect.TypeFor[T]().Elem()
	typ := zero.EventType()
	if _, ok := byType[typ]; ok {
		panic("already registered: " + typ)
	}
	byType[typ] = rt
}

type Event interface {
	EventType() string
}

type RawEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (e *RawEvent) EventType() string {
	return e.Type
}
func (e *RawEvent) Decode() (Event, error) {
	rt, ok := byType[e.Type]
	if !ok {
		return e, nil
	}
	rv := reflect.New(rt)
	err := json.Unmarshal(e.Data, rv.Interface())
	return rv.Interface().(Event), err
}

type eventMsg[T Event] struct {
	Type string `json:"type"`
	Data T      `json:"data"`
}
