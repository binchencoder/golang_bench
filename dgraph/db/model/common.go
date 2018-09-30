package model

import (
	"binchen.com/golang_bench/dgraph/node"
)

type Manager struct {
	DeptIDs []string
	DutyIDs []string
	UserIDs []string
}

type Participator struct {
	DeptIDs []string
	DutyIDs []string
	UserIDs []string
}

type Dept struct {
	*node.Dept
	Manager      *interface{} `json:"manager"`
	Participator *interface{} `json:"participator"`
}

type Duty struct {
	*node.Duty
	Manager      *interface{} `json:"manager"`
	Participator *interface{} `json:"participator"`
}

type User struct {
	*node.User
	Manager      *interface{} `json:"manager"`
	Participator *interface{} `json:"participator"`
}

type Owner struct {
	*node.User
	Owner *interface{} `json:"owner"`
}

type Frame struct {
	*node.Frame
	Parent *interface{} `json:"parent"`
}
