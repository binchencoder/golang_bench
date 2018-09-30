package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	. "binchen.com/golang_bench/dgraph/common"
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
	Depts []struct {
		UID     string `json:"uid"`
		DeptXID string `json:"dept_xid"`
	} `json:"depts"`
	Duties []struct {
		UID     string `json:"uid"`
		DutyXID string `json:"duty_xid"`
	} `json:"duties"`
	Users []struct {
		UID     string `json:"uid"`
		UserXID string `json:"user_xid"`
	} `json:"users"`
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
func (db *IGoalDBImpl) InsertIGoal(manager *node.Manager, participator *node.Participator, creatorID string, igoal node.ParentNode) error {
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
	igoalUID := assigned.Uids["blank-0"]

	orgRoot, err := queryOrgNodesByXIDs(ctx, txn, manager, participator, creatorID)
	if err != nil {
		return err
	}

	depts := orgRoot.Depts
	duties := orgRoot.Duties
	users := orgRoot.Users
	var queryDeptsMap = make(map[string]string)
	var queryDutiesMap = make(map[string]string)
	var queryUsersMap = make(map[string]string)
	if len(depts) > 0 {
		for _, dept := range depts {
			queryDeptsMap[dept.DeptXID] = dept.UID
		}
	}
	if len(duties) > 0 {
		for _, duty := range duties {
			queryDutiesMap[duty.DutyXID] = duty.UID
		}
	}
	if len(users) > 0 {
		for _, user := range users {
			queryUsersMap[user.UserXID] = user.UID
		}
	}

	// Mutate department, duty and user nodes
	queryDeptsMap, err = InsertDeptNodes(ctx, txn, manager, participator, queryDeptsMap)
	checkAndReturnErr(err)
	queryDutiesMap, err = InsertDutyNodes(ctx, txn, manager, participator, queryDutiesMap)
	checkAndReturnErr(err)
	queryUsersMap, err = InsertUserNodes(ctx, txn, manager, participator, creatorID, queryUsersMap)
	checkAndReturnErr(err)

	var set = []*api.NQuad{&api.NQuad{
		Subject:   queryUsersMap[creatorID],
		Predicate: OwnerPredicate,
		ObjectId:  igoalUID,
	}}
	for _, deptXID := range manager.DeptIDs {
		if _, ok := queryDeptsMap[deptXID]; ok {
			set = append(set, &api.NQuad{
				Subject:   queryDeptsMap[deptXID],
				Predicate: ManagerPredicate,
				ObjectId:  igoalUID,
			})
		}
	}
	for _, deptXID := range participator.DeptIDs {
		if _, ok := queryDeptsMap[deptXID]; ok {
			set = append(set, &api.NQuad{
				Subject:   queryDeptsMap[deptXID],
				Predicate: ParticipatorPredicate,
				ObjectId:  igoalUID,
			})
		}
	}
	for _, dutyXID := range manager.DutyIDs {
		if _, ok := queryDutiesMap[dutyXID]; ok {
			set = append(set, &api.NQuad{
				Subject:   queryDutiesMap[dutyXID],
				Predicate: ManagerPredicate,
				ObjectId:  igoalUID,
			})
		}
	}
	for _, dutyXID := range participator.DutyIDs {
		if _, ok := queryDutiesMap[dutyXID]; ok {
			set = append(set, &api.NQuad{
				Subject:   queryDutiesMap[dutyXID],
				Predicate: ParticipatorPredicate,
				ObjectId:  igoalUID,
			})
		}
	}
	for _, userXID := range manager.UserIDs {
		if _, ok := queryUsersMap[userXID]; ok {
			set = append(set, &api.NQuad{
				Subject:   queryUsersMap[userXID],
				Predicate: ManagerPredicate,
				ObjectId:  igoalUID,
			})
		}
	}
	for _, userXID := range participator.UserIDs {
		if _, ok := queryUsersMap[userXID]; ok {
			set = append(set, &api.NQuad{
				Subject:   queryUsersMap[userXID],
				Predicate: ParticipatorPredicate,
				ObjectId:  igoalUID,
			})
		}
	}

	mu = &api.Mutation{}
	mu.Set = set
	assigned, err = txn.Mutate(ctx, mu)
	checkAndReturnErr(err)

	txn.Commit(ctx)

	return nil
}

// InsertDeptNodes inserts department nodes that does not exist.
func InsertDeptNodes(ctx context.Context, txn *dgo.Txn, manager *node.Manager, participator *node.Participator, nodeXIDAndUIDMap map[string]string) (map[string]string, error) {
	var deptNodes = []node.Dept{}
	if len(manager.DeptIDs) > 0 {
		for _, deptXID := range manager.DeptIDs {
			if _, ok := nodeXIDAndUIDMap[deptXID]; !ok {
				deptNodes = append(deptNodes, node.Dept{
					DeptXID: deptXID,
				})
				nodeXIDAndUIDMap[deptXID] = ""
			}
		}
	}
	if len(participator.DeptIDs) > 0 {
		for _, deptXID := range participator.DeptIDs {
			if _, ok := nodeXIDAndUIDMap[deptXID]; !ok {
				deptNodes = append(deptNodes, node.Dept{
					DeptXID: deptXID,
				})
				nodeXIDAndUIDMap[deptXID] = ""
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
			nodeXIDAndUIDMap[deptNodes[i].DeptXID] = assigned.Uids[fmt.Sprintf("blank-%d", i)]
		}
	}

	return nodeXIDAndUIDMap, nil
}

// InsertDutyNodes inserts duty nodes that does not exist.
func InsertDutyNodes(ctx context.Context, txn *dgo.Txn, manager *node.Manager, participator *node.Participator, nodeXIDAndUIDMap map[string]string) (map[string]string, error) {
	var dutyNodes = []node.Duty{}
	if len(manager.DutyIDs) > 0 {
		for _, dutyXID := range manager.DutyIDs {
			if _, ok := nodeXIDAndUIDMap[dutyXID]; !ok {
				dutyNodes = append(dutyNodes, node.Duty{
					DutyXID: dutyXID,
				})
				nodeXIDAndUIDMap[dutyXID] = ""
			}
		}
	}
	if len(participator.DutyIDs) > 0 {
		for _, dutyXID := range participator.DutyIDs {
			if _, ok := nodeXIDAndUIDMap[dutyXID]; !ok {
				dutyNodes = append(dutyNodes, node.Duty{
					DutyXID: dutyXID,
				})
				nodeXIDAndUIDMap[dutyXID] = ""
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
			nodeXIDAndUIDMap[dutyNodes[i].DutyXID] = assigned.Uids[fmt.Sprintf("blank-%d", i)]
		}
	}

	return nodeXIDAndUIDMap, nil
}

// InsertUserNodes inserts user nodes that does not exist.
func InsertUserNodes(ctx context.Context, txn *dgo.Txn, manager *node.Manager, participator *node.Participator, creatorID string, nodeXIDAndUIDMap map[string]string) (map[string]string, error) {
	var userNodes = []node.User{}
	if len(manager.UserIDs) > 0 {
		for _, userXID := range manager.UserIDs {
			if _, ok := nodeXIDAndUIDMap[userXID]; !ok {
				userNodes = append(userNodes, node.User{
					UserXID: userXID,
				})
				nodeXIDAndUIDMap[userXID] = ""
			}
		}
	}
	if len(participator.UserIDs) > 0 {
		for _, userXID := range participator.UserIDs {
			if _, ok := nodeXIDAndUIDMap[userXID]; !ok {
				userNodes = append(userNodes, node.User{
					UserXID: userXID,
				})
				nodeXIDAndUIDMap[userXID] = ""
			}
		}
	}
	if _, ok := nodeXIDAndUIDMap[creatorID]; !ok {
		userNodes = append(userNodes, node.User{
			UserXID: creatorID,
		})
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
			nodeXIDAndUIDMap[userNodes[i].UserXID] = assigned.Uids[fmt.Sprintf("blank-%d", i)]
		}
	}

	return nodeXIDAndUIDMap, nil
}

func queryOrgNodesByXIDs(ctx context.Context, txn *dgo.Txn, manager *node.Manager, participator *node.Participator, creatorID string) (*OrgNodes, error) {
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
