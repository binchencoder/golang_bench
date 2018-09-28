package node

// Dept for graph department node.
type Dept struct {
	DeptXID      string        `json:"dept_xid"`
	Participator []*ParentNode `json:"participator"`
	Manager      []*ParentNode `json:"manager"`
}

// Duty for graph duty node.
type Duty struct {
	DutyXID      string        `json:"duty_xid"`
	Participator []*ParentNode `json:"participator"`
	Manager      []*ParentNode `json:"manager"`
}

// User for graph user node.
type User struct {
	UserXID      string        `json:"user_xid"`
	Participator []*ParentNode `json:"participator"`
	Manager      []*ParentNode `json:"manager"`
	Owner        []*ParentNode `json:"owner"`
}

// Tag for graph tag node.
type Tag struct {
	TagXID   string `json:"tag_xid"`
	Name     string `json:"name"`
	CreateAt int64  `json:"create_at"`
}

type ParentNode interface{}

// IGoal for grapth igoal node.
type IGoal struct {
	ParentNode
	IgoalXID string `json:"igoal_xid"`
	Name     string `json:"name"`
	State    int64  `json:"state"`
	CreateAt int64  `json:"create_at"`
}

// Frame for grapth frame node.
type Frame struct {
	ParentNode
	FrameXID string      `json:"frame_xid"`
	Name     string      `json:"name"`
	Parent   *ParentNode `json:"parent"`
}

// Item for grapth item node.
type Item struct {
	ItemXID string `json:"item_xid"`
	Name    string `json:"name"`
	Parent  *Frame `json:"parent"`
}
