package templater

import "sync"

type NodeType int

const (
	NodeText NodeType = iota
	NodeVariable
	NodeFunction
	NodeIf
	NodeRange
	NodeBlock
	NodeExtends
)

type Node interface {
	Type() NodeType
}

type TextNode struct {
	Content string
}

func (n *TextNode) Type() NodeType { return NodeText }

type VariableNode struct {
	Path string
}

func (n *VariableNode) Type() NodeType { return NodeVariable }

type FunctionNode struct {
	Name      string
	Arguments []string
}

func (n *FunctionNode) Type() NodeType { return NodeFunction }

type IfNode struct {
	Condition  string
	TrueNodes  []Node
	FalseNodes []Node
}

func (n *IfNode) Type() NodeType { return NodeIf }

type RangeNode struct {
	Iterable string
	IndexVar string
	ValueVar string
	Nodes    []Node
}

func (n *RangeNode) Type() NodeType { return NodeRange }

type BlockNode struct {
	Name  string
	Nodes []Node
}

func (n *BlockNode) Type() NodeType { return NodeBlock }

type ExtendsNode struct {
	ParentName string
}

func (n *ExtendsNode) Type() NodeType { return NodeExtends }

type Config struct {
	StrictVariables bool
}

type Template struct {
	Name   string
	Source string
	Nodes  []Node
	Blocks map[string]*BlockNode
	Extends *ExtendsNode
}

type Engine struct {
	mu        sync.RWMutex
	templates map[string]string
	cache     map[string]*Template
	functions map[string]interface{}
	config    Config
}

func NewEngine(config Config) *Engine {
	return &Engine{
		templates: make(map[string]string),
		cache:     make(map[string]*Template),
		functions: make(map[string]interface{}),
		config:    config,
	}
}
