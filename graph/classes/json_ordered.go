package classes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type jsonKV[K ~string, V any] struct {
	Key K
	Val V
}

type jsonMap[K ~string, V any] []jsonKV[K, V]

func (m *jsonMap[K, V]) Append(k K, v V) {
	*m = append(*m, jsonKV[K, V]{Key: k, Val: v})
}

func (m *jsonMap[K, V]) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	if t, err := dec.Token(); err != nil {
		return err
	} else if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("expect JSON object open with '{'")
	}

	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return err
		}

		key, ok := t.(string)
		if !ok {
			return fmt.Errorf("expecting JSON key should be always a string: %T: %v", t, t)
		}
		var val V
		if err = dec.Decode(&val); err != nil {
			return err
		}
		m.Append(K(key), val)
	}

	if t, err := dec.Token(); err != nil {
		return err
	} else if delim, ok := t.(json.Delim); !ok || delim != '}' {
		return fmt.Errorf("expect JSON object close with '}'")
	}
	if t, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("expect end of JSON object but got more token: %T: %v or err: %v", t, t, err)
	}
	return nil
}
