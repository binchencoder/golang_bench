package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"binchen.com/golang_bench/dgraph/db/model"
	"binchen.com/golang_bench/dgraph/node"
	"github.com/dgraph-io/dgo"
	"github.com/dgraph-io/dgo/protos/api"
	"google.golang.org/grpc"
)

type VisibleIGoals struct {
	VisibleIGoals []struct {
		IgoalXID  string `json:"igoal_id"`
		Name      string `json:"name"`
		State     int64  `json:"state"`
		CreatedAt int64  `json:"created_at"`
		Tag       struct {
			TagID string `json:"tag_id"`
			Name  string `json:"name"`
		}
		Creator struct {
			UserID string `json:"user_id"`
		}
	} `json:"visible_igoals"`
}

type OrgNodes struct {
	Depts  []*node.Dept `json:"depts"`
	Duties []*node.Duty `json:"duties"`
	Users  []*node.User `json:"users"`
}

// IGoalDB is defines APIs for igoal node.
type IGoalDB interface {
	// GetVisibleIGoals gets a list of visible igoals.
	GetVisibleIGoals() (*VisibleIGoals, error)
	// InsertIGoal inserts a igoal to graph db.
	InsertIGoal() error
	// GetIGoalTreeAndUsers()
}

// IGoalDBImpl implements IGoalDB interface.
type IGoalDBImpl struct {
	IGoalDB
	connectionStr string
}

// NewIGoalDBImpl creates new IGoalDBImpl object that implements IGoalDB.
func NewIGoalDBImpl(connectionStr string) *IGoalDBImpl {
	return &IGoalDBImpl{
		connectionStr: connectionStr,
	}
}

// GetVisibleIGoals gets a list of visible igoals.
func (db *IGoalDBImpl) GetVisibleIGoals(deptIDs []string, dutyIDs []string, userIDs []string) (*VisibleIGoals, error) {
	conn, err := grpc.Dial(db.connectionStr, grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial dgraph gRPC")
	}
	defer conn.Close()

	dc := api.NewDgraphClient(conn)
	dg := dgo.NewDgraphClient(dc)

	var deptXIDs = ""
	var dutyXIDs = ""
	var userXIDs = ""
	if len(deptIDs) > 0 {
		deptXIDs = strings.Join(deptIDs, " ")
	}
	if len(dutyIDs) > 0 {
		dutyXIDs = strings.Join(dutyIDs, " ")
	}
	if len(userIDs) > 0 {
		userXIDs = strings.Join(userIDs, " ")
	}

	variables := make(map[string]string)
	variables["$dept_xid"] = deptXIDs
	variables["$duty_xid"] = dutyXIDs
	variables["$user_xid"] = userXIDs

	const q = `query Visible_igoals($dept_xid: string, $duty_xid: string, $user_xid: string){
		var(func: anyofterms(dept_xid, $dept_xid)) {
			dept_uids as uid
		}
	
		var(func: anyofterms(duty_xid, $duty_xid)) {
			duty_uids as uid
		}
	
		var(func: anyofterms(user_xid, $user_xid)) {
			user_uids as uid
		}
	
		var(func: uid(dept_uids, duty_uids, user_uids)) @recurse(depth: 10, loop: true) {
			igoal_uid as igoal_xid
			participator {
				igoal_xid
				parent {
					igoal_xid
				}
			}
			manager {
				igoal_xid
				parent {
					igoal_xid
				}
			}
			owner {
				igoal_xid
			}
			parent {
				igoal_xid
			}
		}
	
		visible_igoals(func: uid(igoal_uid), orderasc: updated_at) {
			igoal_id: igoal_xid
			name
			created_at
			state
			tag: tag_of {
				tag_id: tag_xid
				name
			}
			creator: ~owner {
				user_id: user_xid
			}
		}
	}`

	ctx := context.Background()
	resp, err := dg.NewTxn().QueryWithVars(ctx, q, variables)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	fmt.Println(string(resp.GetJson()))

	var r VisibleIGoals
	err = json.Unmarshal(resp.GetJson(), &r)

	return &r, err
}

// InsertIGoal inserts a igoal to graph db.
func (db *IGoalDBImpl) InsertIGoal(manager *model.Manager, participator *model.Participator, creatorID string, igoal interface{}) error {
	conn, err := grpc.Dial(db.connectionStr, grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial dgraph gRPC")
	}
	defer conn.Close()

	dc := api.NewDgraphClient(conn)
	dg := dgo.NewDgraphClient(dc)

	txn := dg.NewTxn()
	ctx := context.Background()
	mu := &api.Mutation{}

	// Mutate igoal node
	igoalPb, err := json.Marshal(igoal)
	if err != nil {
		log.Fatal(err)
		return err
	}

	mu.SetJson = igoalPb
	assigned, err := txn.Mutate(ctx, mu)
	if err != nil {
		log.Fatal(err)
		return err
	}

	val, ok := igoal.(*node.IGoal)
	if ok {
		val.UID = assigned.Uids["blank-0"]
	}

	orgRoot, err := queryOrgNodesByXIDs(ctx, txn, manager, participator, creatorID)
	if err != nil {
		return err
	}

	depts := orgRoot.Depts
	duties := orgRoot.Duties
	users := orgRoot.Users
	var queryDeptsMap = make(map[string]*node.Dept)
	var queryDutiesMap = make(map[string]*node.Duty)
	var queryUsersMap = make(map[string]*node.User)
	if len(depts) > 0 {
		for _, dept := range depts {
			queryDeptsMap[dept.DeptXID] = dept
		}
	}
	if len(duties) > 0 {
		for _, duty := range duties {
			queryDutiesMap[duty.DutyXID] = duty
		}
	}
	if len(users) > 0 {
		for _, user := range users {
			queryUsersMap[user.UserXID] = user
		}
	}

	// Mutate department, duty and user nodes
	queryDeptsMap, err = InsertDeptNodes(ctx, txn, manager, participator, queryDeptsMap)
	checkAndReturnErr(err)
	queryDutiesMap, err = InsertDutyNodes(ctx, txn, manager, participator, queryDutiesMap)
	checkAndReturnErr(err)
	queryUsersMap, err = InsertUserNodes(ctx, txn, manager, participator, creatorID, queryUsersMap)
	checkAndReturnErr(err)

	var deptSet = []*model.Dept{}
	var dutySet = []*model.Duty{}
	var userSet = []*model.User{}
	for _, deptXID := range manager.DeptIDs {
		if _, ok := queryDeptsMap[deptXID]; ok {
			deptSet = append(deptSet, &model.Dept{
				Dept:    queryDeptsMap[deptXID],
				Manager: &igoal,
			})
		}
	}
	for _, deptXID := range participator.DeptIDs {
		if _, ok := queryDeptsMap[deptXID]; ok {
			deptSet = append(deptSet, &model.Dept{
				Dept:         queryDeptsMap[deptXID],
				Participator: &igoal,
			})
		}
	}
	mu = &api.Mutation{}
	deptsPb, err := json.Marshal(deptSet)
	mu.SetJson = deptsPb
	_, err = txn.Mutate(ctx, mu)
	checkAndReturnErr(err)

	for _, dutyXID := range manager.DutyIDs {
		if _, ok := queryDutiesMap[dutyXID]; ok {
			dutySet = append(dutySet, &model.Duty{
				Duty:    queryDutiesMap[dutyXID],
				Manager: &igoal,
			})
		}
	}
	for _, dutyXID := range participator.DutyIDs {
		if _, ok := queryDutiesMap[dutyXID]; ok {
			dutySet = append(dutySet, &model.Duty{
				Duty:         queryDutiesMap[dutyXID],
				Participator: &igoal,
			})
		}
	}
	mu = &api.Mutation{}
	dutiesPb, err := json.Marshal(dutySet)
	mu.SetJson = dutiesPb
	_, err = txn.Mutate(ctx, mu)
	checkAndReturnErr(err)

	for _, userXID := range manager.UserIDs {
		if _, ok := queryUsersMap[userXID]; ok {
			userSet = append(userSet, &model.User{
				User:    queryUsersMap[userXID],
				Manager: &igoal,
			})
		}
	}
	for _, userXID := range participator.UserIDs {
		if _, ok := queryUsersMap[userXID]; ok {
			userSet = append(userSet, &model.User{
				User:         queryUsersMap[userXID],
				Participator: &igoal,
			})
		}
	}
	mu = &api.Mutation{}
	usersPb, err := json.Marshal(userSet)
	checkAndReturnErr(err)
	mu.SetJson = usersPb
	_, err = txn.Mutate(ctx, mu)
	checkAndReturnErr(err)

	owner := &model.Owner{
		User:  queryUsersMap[creatorID],
		Owner: &igoal,
	}
	mu = &api.Mutation{}
	ownerPb, err := json.Marshal(owner)
	checkAndReturnErr(err)
	mu.SetJson = ownerPb
	_, err = txn.Mutate(ctx, mu)
	checkAndReturnErr(err)

	txn.Commit(ctx)
	return nil
}

// InsertDeptNodes inserts department nodes that does not exist.
func InsertDeptNodes(ctx context.Context, txn *dgo.Txn, manager *model.Manager, participator *model.Participator, nodeXIDAndUIDMap map[string]*node.Dept) (map[string]*node.Dept, error) {
	var deptNodes = []*node.Dept{}
	if len(manager.DeptIDs) > 0 {
		for _, deptXID := range manager.DeptIDs {
			if _, ok := nodeXIDAndUIDMap[deptXID]; !ok {
				dept := &node.Dept{
					DeptXID: deptXID,
				}
				deptNodes = append(deptNodes, dept)
				nodeXIDAndUIDMap[deptXID] = dept
			}
		}
	}
	if len(participator.DeptIDs) > 0 {
		for _, deptXID := range participator.DeptIDs {
			if _, ok := nodeXIDAndUIDMap[deptXID]; !ok {
				dept := &node.Dept{
					DeptXID: deptXID,
				}
				deptNodes = append(deptNodes, dept)
				nodeXIDAndUIDMap[deptXID] = dept
			}
		}
	}
	if len(deptNodes) > 0 {
		deptNodesPb, err := json.Marshal(deptNodes)
		if err != nil {
			return nodeXIDAndUIDMap, err
		}

		mu := &api.Mutation{}
		mu.SetJson = deptNodesPb
		assigned, err := txn.Mutate(ctx, mu)
		if err != nil {
			log.Fatal(err)
			return nodeXIDAndUIDMap, err
		}
		for i := 0; i < len(deptNodes); i++ {
			nodeXIDAndUIDMap[deptNodes[i].DeptXID].UID = assigned.Uids[fmt.Sprintf("blank-%d", i)]
		}
	}

	return nodeXIDAndUIDMap, nil
}

// InsertDutyNodes inserts duty nodes that does not exist.
func InsertDutyNodes(ctx context.Context, txn *dgo.Txn, manager *model.Manager, participator *model.Participator, nodeXIDAndUIDMap map[string]*node.Duty) (map[string]*node.Duty, error) {
	var dutyNodes = []*node.Duty{}
	if len(manager.DutyIDs) > 0 {
		for _, dutyXID := range manager.DutyIDs {
			if _, ok := nodeXIDAndUIDMap[dutyXID]; !ok {
				duty := &node.Duty{
					DutyXID: dutyXID,
				}
				dutyNodes = append(dutyNodes, duty)
				nodeXIDAndUIDMap[dutyXID] = duty
			}
		}
	}
	if len(participator.DutyIDs) > 0 {
		for _, dutyXID := range participator.DutyIDs {
			if _, ok := nodeXIDAndUIDMap[dutyXID]; !ok {
				duty := &node.Duty{
					DutyXID: dutyXID,
				}
				dutyNodes = append(dutyNodes, duty)
				nodeXIDAndUIDMap[dutyXID] = duty
			}
		}
	}
	if len(dutyNodes) > 0 {
		dutiesNodesPb, err := json.Marshal(dutyNodes)
		if err != nil {
			return nodeXIDAndUIDMap, err
		}

		mu := &api.Mutation{}
		mu.SetJson = dutiesNodesPb
		assigned, err := txn.Mutate(ctx, mu)
		if err != nil {
			log.Fatal(err)
			return nodeXIDAndUIDMap, err
		}
		for i := 0; i < len(dutyNodes); i++ {
			nodeXIDAndUIDMap[dutyNodes[i].DutyXID].UID = assigned.Uids[fmt.Sprintf("blank-%d", i)]
		}
	}

	return nodeXIDAndUIDMap, nil
}

// InsertUserNodes inserts user nodes that does not exist.
func InsertUserNodes(ctx context.Context, txn *dgo.Txn, manager *model.Manager, participator *model.Participator, creatorID string, nodeXIDAndUIDMap map[string]*node.User) (map[string]*node.User, error) {
	var userNodes = []*node.User{}
	if len(manager.UserIDs) > 0 {
		for _, userXID := range manager.UserIDs {
			if _, ok := nodeXIDAndUIDMap[userXID]; !ok {
				user := &node.User{
					UserXID: userXID,
				}
				userNodes = append(userNodes, user)
				nodeXIDAndUIDMap[userXID] = user
			}
		}
	}
	if len(participator.UserIDs) > 0 {
		for _, userXID := range participator.UserIDs {
			if _, ok := nodeXIDAndUIDMap[userXID]; !ok {
				user := &node.User{
					UserXID: userXID,
				}
				userNodes = append(userNodes, user)
				nodeXIDAndUIDMap[userXID] = user
			}
		}
	}
	if _, ok := nodeXIDAndUIDMap[creatorID]; !ok {
		user := &node.User{
			UserXID: creatorID,
		}
		userNodes = append(userNodes, user)
		nodeXIDAndUIDMap[creatorID] = user
	}

	if len(userNodes) > 0 {
		userNodesPb, err := json.Marshal(userNodes)
		if err != nil {
			return nodeXIDAndUIDMap, err
		}

		mu := &api.Mutation{}
		mu.SetJson = userNodesPb
		assigned, err := txn.Mutate(ctx, mu)
		if err != nil {
			log.Fatal(err)
			return nodeXIDAndUIDMap, err
		}
		for i := 0; i < len(userNodes); i++ {
			nodeXIDAndUIDMap[userNodes[i].UserXID].UID = assigned.Uids[fmt.Sprintf("blank-%d", i)]
		}
	}

	return nodeXIDAndUIDMap, nil
}

func queryOrgNodesByXIDs(ctx context.Context, txn *dgo.Txn, manager *model.Manager, participator *model.Participator, creatorID string) (*OrgNodes, error) {
	var deptXIDs, dutyXIDs, userXIDs = "", "", ""

	// 负责人
	if len(manager.DeptIDs) > 0 {
		deptXIDs = strings.Join(manager.DeptIDs, " ")
	}
	if len(manager.DutyIDs) > 0 {
		dutyXIDs = strings.Join(manager.DutyIDs, " ")
	}
	if len(manager.UserIDs) > 0 {
		userXIDs = strings.Join(manager.UserIDs, " ")
	}

	// 参与人
	parDeptIDs := participator.DeptIDs
	if len(parDeptIDs) > 0 {
		deptXIDs += " " + strings.Join(parDeptIDs, " ")
	}
	parDutyIDs := participator.DutyIDs
	if len(parDutyIDs) > 0 {
		dutyXIDs += " " + strings.Join(parDutyIDs, " ")
	}
	parUserIDs := participator.UserIDs
	if len(parUserIDs) > 0 {
		userXIDs += " " + strings.Join(parUserIDs, " ")
	}

	// Owner
	userXIDs += " " + creatorID

	variables := make(map[string]string)
	variables["$dept_xid"] = deptXIDs
	variables["$duty_xid"] = dutyXIDs
	variables["$user_xid"] = userXIDs

	const q = `query UIDs_by_xids($dept_xid: string, $duty_xid: string, $user_xid: string){
		var(func: anyofterms(dept_xid, $dept_xid)) {
			dept_uids as uid
		}
	
		var(func: anyofterms(duty_xid, $duty_xid)) {
			duty_uids as uid
		}
	
		var(func: anyofterms(user_xid, $user_xid)) {
			user_uids as uid
		}

		depts(func: uid(dept_uids)) {
			uid
			dept_xid
		}

		duties(func: uid(duty_uids)) {
			uid
			duty_xid
		}

		users(func: uid(user_uids)) {
			uid
			user_xid
		}
	}`

	resp, err := txn.QueryWithVars(ctx, q, variables)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(resp.GetJson()))

	var root OrgNodes
	err = json.Unmarshal(resp.GetJson(), &root)
	return &root, err
}

func checkAndReturnErr(err error) error {
	if err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}
