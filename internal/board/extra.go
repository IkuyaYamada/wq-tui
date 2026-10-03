package board

import (
	"encoding/json"
	"reflect"
	"strings"
)

// Fields this build does not know about are kept and written back, so an
// older wq still running beside a newer one never drops what the newer one
// added to board.json.

func (n *Node) UnmarshalJSON(data []byte) error {
	type plain Node
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	extra, err := unknownFields(data, reflect.TypeOf(p))
	if err != nil {
		return err
	}
	*n = Node(p)
	n.extra = extra
	return nil
}

func (n Node) MarshalJSON() ([]byte, error) {
	type plain Node
	return withExtra(plain(n), n.extra)
}

func (b *Board) UnmarshalJSON(data []byte) error {
	type plain Board
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	extra, err := unknownFields(data, reflect.TypeOf(p))
	if err != nil {
		return err
	}
	*b = Board(p)
	b.extra = extra
	return nil
}

func (b Board) MarshalJSON() ([]byte, error) {
	type plain Board
	return withExtra(plain(b), b.extra)
}

// unknownFields returns the members of a JSON object that no json tag of t
// claims, or nil if there are none.
func unknownFields(data []byte, t reflect.Type) (map[string]json.RawMessage, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		delete(all, name)
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all, nil
}

// withExtra marshals v and adds the kept unknown members back.
func withExtra(v any, extra map[string]json.RawMessage) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil || len(extra) == 0 {
		return data, err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	for k, raw := range extra {
		if _, known := all[k]; !known {
			all[k] = raw
		}
	}
	return json.Marshal(all)
}
