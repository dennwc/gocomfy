package types

import (
	"strconv"
)

type NodeID uint

func (id NodeID) String() string {
	return strconv.FormatUint(uint64(id), 10)
}

func (id *NodeID) Parse(s string) error {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return err
	}
	*id = NodeID(v)
	return nil
}

type NodeClass string

type TypeName string

func (t TypeName) IsScalar() bool {
	switch t {
	case IntType, FloatType, StringType, BoolType:
		return true
	}
	return false
}

const (
	IntType    = TypeName("INT")
	StringType = TypeName("STRING")
	FloatType  = TypeName("FLOAT")
	BoolType   = TypeName("BOOLEAN")
)
