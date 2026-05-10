package generator

type Expr string

type ValueType uint8

const (
	ValueFloat64 ValueType = iota
	ValueInt64
	ValueUint64
	ValueBool
)

type Model struct {
	Hash string

	Entities  []*EntityDesc
	Vars      []*VarDesc
	Resources []*ResourceDesc
	Blocks    []*BlockDesc

	EntityByName   map[string]*EntityDesc
	VarByName      map[string]*VarDesc
	ResourceByName map[string]*ResourceDesc
	BlockByName    map[string]*BlockDesc
}

func NewModel() *Model {
	return &Model{
		EntityByName:   make(map[string]*EntityDesc),
		VarByName:      make(map[string]*VarDesc),
		ResourceByName: make(map[string]*ResourceDesc),
		BlockByName:    make(map[string]*BlockDesc),
	}
}

type EntityDesc struct {
	Name string

	Fields      []*FieldDesc
	FieldByName map[string]*FieldDesc
}

type FieldDesc struct {
	Name    string
	Type    ValueType
	Default string
}

type VarDesc struct {
	Name    string
	Type    ValueType
	Default string
}

type ResourceDesc struct {
	Name string

	Capacity Expr

	SeizeNames []string
	Seizes     []*BlockDesc
}

type BlockType uint8

type BlockDesc struct {
	Name string

	Params BlockParams
}

type BlockParams interface {
	isBlockParams()
}

type BlockingPolicy uint8

const (
	BlockingPolicyError BlockingPolicy = iota
	BlockingPolicyDestroy
)

type QueueDiscipline uint8

const (
	QueueDisciplineFIFO QueueDiscipline = iota
)

type CreateParams struct {
	EntityName string
	Entity     *EntityDesc

	Interval Expr

	Immediately bool

	Batch Expr

	BlockingPolicy BlockingPolicy

	NextName string
	Next     *BlockDesc
}

func (*CreateParams) isBlockParams() {}

type QueueParams struct {
	Capacity Expr

	Discipline QueueDiscipline

	Timeout    Expr
	HasTimeout bool

	NextTimeoutName string
	NextTimeout     *BlockDesc

	ExitOnOverflow bool

	NextOverflowName string
	NextOverflow     *BlockDesc

	NextName string
	Next     *BlockDesc
}

func (*QueueParams) isBlockParams() {}

type DelayParams struct {
	Duration Expr

	Capacity    Expr
	HasCapacity bool

	WaitQueueNames []string
	WaitQueues     []*BlockDesc

	NextName string
	Next     *BlockDesc
}

func (*DelayParams) isBlockParams() {}

type SeizeParams struct {
	ResourceName string
	Resource     *ResourceDesc

	WaitQueueNames []string
	WaitQueues     []*BlockDesc

	NextName string
	Next     *BlockDesc
}

func (*SeizeParams) isBlockParams() {}

type ReleaseParams struct {
	ResourceName string
	Resource     *ResourceDesc

	NextName string
	Next     *BlockDesc
}

func (*ReleaseParams) isBlockParams() {}

type AssignParams struct {
	Assignments []Assignment

	NextName string
	Next     *BlockDesc
}

func (*AssignParams) isBlockParams() {}

type Assignment struct {
	Target AssignmentTarget
	Expr   Expr
}

type AssignmentTarget struct {
	IsEntityField bool

	EntityName string
	FieldName  string

	VarName string
}

type BranchParams struct {
	Cases []BranchCase

	HasElse  bool
	ElseName string
	Else     *BlockDesc
}

func (*BranchParams) isBlockParams() {}

type BranchCase struct {
	Condition Expr

	NextName string
	Next     *BlockDesc
}

type TerminateParams struct{}

func (*TerminateParams) isBlockParams() {}
