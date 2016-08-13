// Package neuraltree implements a neural architecture
// in which small neural networks are arranged as nodes
// in a tree.
package neuraltree

import (
	"errors"

	"github.com/unixpickle/autofunc"
	"github.com/unixpickle/serializer"
	"github.com/unixpickle/weakai/neuralnet"
)

func init() {
	var n Node
	serializer.RegisterTypedDeserializer(n.SerializerType(), DeserializeNode)
}

// A Node makes decisions by running sub-nodes or
// evaluating a leaf neural network.
type Node struct {
	Network  neuralnet.Network
	Children []*Node
}

// DeserializeNode deserializes a Node.
func DeserializeNode(d []byte) (*Node, error) {
	list, err := serializer.DeserializeSlice(d)
	if err != nil {
		return nil, err
	} else if len(list) == 0 {
		return nil, errors.New("invalid Node slice")
	}
	res := &Node{Children: make([]*Node, len(list)-1)}
	var ok bool
	res.Network = list[len(list)-1].(neuralnet.Network)
	if !ok {
		return nil, errors.New("invalid Node slice")
	}
	for i, child := range list[:len(list)-1] {
		res.Children[i], ok = child.(*Node)
		if !ok {
			return nil, errors.New("invalid Node slice")
		}
	}
	return res, nil
}

// NewNodeBinTree creates a binary tree.
// The depth specifies the number of non-leaf layers
// of the tree, so a depth of 0 implies a single node.
func NewNodeBinTree(depth, inSize, hiddenSize, classCount int) *Node {
	if depth == 0 {
		net := neuralnet.Network{
			&neuralnet.DenseLayer{
				InputCount:  inSize,
				OutputCount: hiddenSize,
			},
			&neuralnet.HyperbolicTangent{},
			&neuralnet.DenseLayer{
				InputCount:  hiddenSize,
				OutputCount: classCount,
			},
			&neuralnet.SoftmaxLayer{},
		}
		net.Randomize()
		return &Node{
			Network: net,
		}
	}

	net := neuralnet.Network{
		&neuralnet.DenseLayer{
			InputCount:  inSize,
			OutputCount: hiddenSize,
		},
		&neuralnet.HyperbolicTangent{},
		&neuralnet.DenseLayer{
			InputCount:  hiddenSize,
			OutputCount: 2,
		},
		&neuralnet.SoftmaxLayer{},
	}
	net.Randomize()
	return &Node{
		Network: net,
		Children: []*Node{
			NewNodeBinTree(depth-1, inSize, hiddenSize, classCount),
			NewNodeBinTree(depth-1, inSize, hiddenSize, classCount),
		},
	}
}

// Apply runs the node on a given input, returning a
// vector of averaged decision leaves.
func (n *Node) Apply(input autofunc.Result) autofunc.Result {
	decisionWeights := n.Network.Apply(input)
	if len(n.Children) == 0 {
		return decisionWeights
	}
	if len(decisionWeights.Output()) != len(n.Children) {
		panic("child node count must match network output size")
	}
	return autofunc.Pool(decisionWeights, func(w autofunc.Result) autofunc.Result {
		var res autofunc.Result
		for i := 0; i < len(w.Output()); i++ {
			weight := autofunc.Slice(w, i, i+1)
			childOut := n.Children[i].Apply(input)
			weighted := autofunc.ScaleFirst(childOut, weight)
			if res == nil {
				res = weighted
			} else {
				res = autofunc.Add(res, weighted)
			}
		}
		return res
	})
}

// ApplyR is the r-operator version of Apply.
func (n *Node) ApplyR(rv autofunc.RVector, input autofunc.RResult) autofunc.RResult {
	decisionWeights := n.Network.ApplyR(rv, input)
	if len(n.Children) == 0 {
		return decisionWeights
	}
	if len(decisionWeights.Output()) != len(n.Children) {
		panic("child node count must match network output size")
	}
	return autofunc.PoolR(decisionWeights, func(w autofunc.RResult) autofunc.RResult {
		var res autofunc.RResult
		for i := 0; i < len(w.Output()); i++ {
			weight := autofunc.SliceR(w, i, i+1)
			childOut := n.Children[i].ApplyR(rv, input)
			weighted := autofunc.ScaleFirstR(childOut, weight)
			if res == nil {
				res = weighted
			} else {
				res = autofunc.AddR(res, weighted)
			}
		}
		return res
	})
}

// Parameters returns all of the parameters of the node's
// network, as well as those of its children.
func (n *Node) Parameters() []*autofunc.Variable {
	var res []*autofunc.Variable
	res = append(res, n.Network.Parameters()...)
	for _, child := range n.Children {
		res = append(res, child.Parameters()...)
	}
	return res
}

// SerializerType returns the unique ID for serializing
// nodes with the serializer package.
func (n *Node) SerializerType() string {
	return "github.com/unixpickle/neuraltree.Node"
}

// Serialize serializes the node.
func (n *Node) Serialize() ([]byte, error) {
	list := make([]serializer.Serializer, len(n.Children)+1)
	for i, c := range n.Children {
		list[i] = c
	}
	list[len(list)-1] = n.Network
	return serializer.SerializeSlice(list)
}