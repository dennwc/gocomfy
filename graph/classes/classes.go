package classes

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dennwc/gocomfy/graph/types"
)

func Decode(r io.Reader) (Classes, error) {
	var res map[types.NodeClass]jsonClass
	if err := json.NewDecoder(r).Decode(&res); err != nil {
		return nil, err
	}
	return convertClasses(res)
}

type Classes map[types.NodeClass]*Class

type Class struct {
	Name     types.NodeClass
	Title    string
	Desc     string
	Category string
	IsOutput bool
	Inputs   []Input
	Outputs  []Output
}

type InputKind int

func (k InputKind) String() string {
	switch k {
	case InputRequired:
		return "Required"
	case InputOptional:
		return "Optional"
	case InputHidden:
		return "Hidden"
	default:
		return fmt.Sprintf("Kind(%d)", int(k))
	}
}

const (
	InputRequired = InputKind(iota)
	InputOptional
	InputHidden
)

type Input struct {
	Name     string
	Kind     InputKind
	Type     types.TypeName
	Config   json.RawMessage
	IsSelect bool
	Select   []Option
}

type Option struct {
	Name   string
	Inputs []Input
}

type Output struct {
	Name   string
	Type   types.TypeName
	OneOf  []types.TypeName
	IsList bool
}

type jsonClass struct {
	Name      types.NodeClass  `json:"name"`
	Title     string           `json:"display_name"`
	Desc      string           `json:"description"`
	Category  string           `json:"category"`
	IsOutput  bool             `json:"output_node"`
	OutTypes  []jsonTypeOutput `json:"output"`
	OutNames  []string         `json:"output_name"`
	OutIsList []bool           `json:"output_is_list"`
	Input     struct {
		Required jsonMap[string, jsonTypeInput]  `json:"required"`
		Optional jsonMap[string, jsonTypeInput]  `json:"optional"`
		Hidden   jsonMap[string, types.TypeName] `json:"hidden"`
	} `json:"input"`
}

var _ json.Unmarshaler = (*jsonOption)(nil)

type jsonOption struct {
	Name   string
	Inputs jsonMap[string, jsonTypeInput]
}

func (v *jsonOption) UnmarshalJSON(data []byte) error {
	*v = jsonOption{}
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		v.Name = name
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	if err := json.Unmarshal(arr[0], &name); err != nil {
		return err
	}
	v.Name = name
	v.Inputs = make(jsonMap[string, jsonTypeInput], 0, len(arr)-1)
	for _, raw := range arr[1:] {
		var iarr []json.RawMessage
		if err := json.Unmarshal(raw, &iarr); err != nil {
			return err
		}
		for _, iraw := range iarr {
			var inarr []json.RawMessage
			if err := json.Unmarshal(iraw, &inarr); err != nil {
				return err
			}
			var iname string
			if err := json.Unmarshal(inarr[0], &iname); err != nil {
				return err
			}
			var inp jsonTypeInput
			if err := inp.UnmarshalJSONArray(inarr[1:]); err != nil {
				return err
			}
			v.Inputs.Append(iname, inp)
		}
	}
	return nil
}

var _ json.Unmarshaler = (*jsonTypeInput)(nil)

type jsonTypeInput struct {
	Type   types.TypeName
	Select []jsonOption
	Config json.RawMessage
}

func (v *jsonTypeInput) UnmarshalJSONArray(arr []json.RawMessage) error {
	*v = jsonTypeInput{}
	if len(arr) != 1 && len(arr) != 2 {
		return fmt.Errorf("invalid input size: %d", len(arr))
	}
	if len(arr) >= 2 {
		v.Config = arr[1]
	}
	val := arr[0]
	var typ types.TypeName
	if err := json.Unmarshal(val, &typ); err == nil {
		v.Type = typ
		return nil
	}
	var sel []jsonOption
	err := json.Unmarshal(val, &sel)
	if err != nil {
		return fmt.Errorf("cannot unmarshal options: %w", err)
	}
	v.Select = sel
	return nil
}

func (v *jsonTypeInput) UnmarshalJSON(data []byte) error {
	*v = jsonTypeInput{}
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	return v.UnmarshalJSONArray(arr)
}

func (v *jsonTypeInput) Convert(name string, kind InputKind) Input {
	var sel []Option
	if len(v.Select) != 0 {
		sel = make([]Option, 0, len(v.Select))
		for _, opt := range v.Select {
			var inps []Input
			if len(opt.Inputs) != 0 {
				inps = make([]Input, 0, len(opt.Inputs))
				for _, ikv := range opt.Inputs {
					inps = append(inps, ikv.Val.Convert(ikv.Key, kind))
				}
			}
			sel = append(sel, Option{
				Name:   opt.Name,
				Inputs: inps,
			})
		}
	}
	typ := v.Type
	if typ == "" {
		typ = types.StringType
	}
	return Input{
		Name:     name,
		Kind:     kind,
		Type:     typ,
		Config:   v.Config,
		Select:   sel,
		IsSelect: v.Select != nil,
	}
}

var _ json.Unmarshaler = (*jsonTypeOutput)(nil)

type jsonTypeOutput struct {
	Type  types.TypeName
	OneOf []types.TypeName
}

func (v *jsonTypeOutput) UnmarshalJSON(data []byte) error {
	*v = jsonTypeOutput{}
	var typ types.TypeName
	if err := json.Unmarshal(data, &typ); err == nil {
		v.Type = typ
		return nil
	}
	var oneof []types.TypeName
	if err := json.Unmarshal(data, &oneof); err != nil {
		return err
	}
	v.OneOf = oneof
	return nil
}

func convertClasses(res map[types.NodeClass]jsonClass) (map[types.NodeClass]*Class, error) {
	out := make(map[types.NodeClass]*Class, len(res))
	for key, jobj := range res {
		obj := &Class{
			Name:     jobj.Name,
			Title:    jobj.Title,
			Desc:     jobj.Desc,
			Category: jobj.Category,
			Inputs:   make([]Input, 0, len(jobj.Input.Required)+len(jobj.Input.Optional)+len(jobj.Input.Hidden)),
			Outputs:  make([]Output, 0, len(jobj.OutTypes)),
			IsOutput: false,
		}
		out[key] = obj
		for _, kv := range jobj.Input.Required {
			obj.Inputs = append(obj.Inputs, kv.Val.Convert(kv.Key, InputRequired))
		}
		for _, kv := range jobj.Input.Optional {
			obj.Inputs = append(obj.Inputs, kv.Val.Convert(kv.Key, InputOptional))
		}
		for _, kv := range jobj.Input.Hidden {
			obj.Inputs = append(obj.Inputs, Input{
				Name: kv.Key,
				Kind: InputHidden,
				Type: kv.Val,
			})
		}
		for i, typ := range jobj.OutTypes {
			obj.Outputs = append(obj.Outputs, Output{
				Type:   typ.Type,
				OneOf:  typ.OneOf,
				Name:   jobj.OutNames[i],
				IsList: jobj.OutIsList[i],
			})
		}
	}
	return out, nil
}
