package db

import (
	"dgraph/node"
)

// IGoalDB is defines APIs for igoal node.
type IGoalDB interface {
	GetVisibleIGoals() ([]*node.IGoal, error)
	InsertIGoal() error
}
