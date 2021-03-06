package db

import (
	"fmt"
	"testing"
	"time"

	"binchen.com/golang_bench/dgraph/db/model"
	"binchen.com/golang_bench/dgraph/node"

	. "binchen.com/golang_bench/dgraph/common"
)

func TestGetVisibleIGoals(t *testing.T) {
	igoalDBImpl := NewIGoalDBImpl(DgraphServerPoint)

	deptIDs := []string{" "}
	dutyIDs := []string{" "}
	userIDs := []string{"179"}
	igoals, err := igoalDBImpl.GetVisibleIGoals(deptIDs, dutyIDs, userIDs)
	if err != nil {
		fmt.Println(igoals)
	}
}

func TestInsertIGoal(t *testing.T) {
	igoalDBImpl := NewIGoalDBImpl(DgraphServerPoint)

	deptIDs := []string{"4", "5"}
	dutyIDs := []string{"105", "106", "107"}
	userIDs := []string{"206", "207"}

	manager := &model.Manager{
		DeptIDs: deptIDs,
		DutyIDs: dutyIDs,
		UserIDs: userIDs,
	}
	participator := &model.Participator{
		DeptIDs: deptIDs,
		DutyIDs: dutyIDs,
		UserIDs: userIDs,
	}

	now := int64(time.Now().UTC().Nanosecond())
	igoal := &node.IGoal{
		IgoalXID:  "1004",
		Name:      "IGoal 1004",
		State:     1,
		CreatedAt: now,
	}

	igoalDBImpl.InsertIGoal(manager, participator, "206", igoal)
}
