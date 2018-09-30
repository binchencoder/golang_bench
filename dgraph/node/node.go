package node

// Dept for graph department node.
type Dept struct {
	UID     string `json:"uid,omitempty"`
	DeptXID string `json:"dept_xid"`
}

// Duty for graph duty node.
type Duty struct {
	UID     string `json:"uid,omitempty"`
	DutyXID string `json:"duty_xid"`
}

// User for graph user node.
type User struct {
	UID     string `json:"uid,omitempty"`
	UserXID string `json:"user_xid"`
}

// Tag for graph tag node.
type Tag struct {
	UID       string `json:"uid,omitempty"`
	TagXID    string `json:"tag_xid"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
}

type ParentNode interface{}

// IGoal for grapth igoal node.
type IGoal struct {
	ParentNode
	UID       string `json:"uid,omitempty"`
	IgoalXID  string `json:"igoal_xid"`
	Name      string `json:"name"`
	State     int64  `json:"state"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// Frame for grapth frame node.
type Frame struct {
	ParentNode
	UID      string `json:"uid,omitempty"`
	FrameXID string `json:"frame_xid"`
	Name     string `json:"name"`
}

// Item for grapth item node.
type Item struct {
	UID     string `json:"uid,omitempty"`
	ItemXID string `json:"item_xid"`
	Name    string `json:"name"`
}

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
