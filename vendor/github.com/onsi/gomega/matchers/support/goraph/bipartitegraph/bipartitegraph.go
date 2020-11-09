package bipartitegraph

<<<<<<< HEAD
=======
import "errors"
>>>>>>> cbc9bb05... fixup add vendor back
import "fmt"

import . "github.com/onsi/gomega/matchers/support/goraph/node"
import . "github.com/onsi/gomega/matchers/support/goraph/edge"

type BipartiteGraph struct {
	Left  NodeOrderedSet
	Right NodeOrderedSet
	Edges EdgeSet
}

func NewBipartiteGraph(leftValues, rightValues []interface{}, neighbours func(interface{}, interface{}) (bool, error)) (*BipartiteGraph, error) {
	left := NodeOrderedSet{}
	for i := range leftValues {
		left = append(left, Node{Id: i})
	}

	right := NodeOrderedSet{}
	for j := range rightValues {
		right = append(right, Node{Id: j + len(left)})
	}

	edges := EdgeSet{}
	for i, leftValue := range leftValues {
		for j, rightValue := range rightValues {
			neighbours, err := neighbours(leftValue, rightValue)
			if err != nil {
<<<<<<< HEAD
				return nil, fmt.Errorf("error determining adjacency for %v and %v: %s", leftValue, rightValue, err.Error())
=======
				return nil, errors.New(fmt.Sprintf("error determining adjacency for %v and %v: %s", leftValue, rightValue, err.Error()))
>>>>>>> cbc9bb05... fixup add vendor back
			}

			if neighbours {
				edges = append(edges, Edge{Node1: left[i], Node2: right[j]})
			}
		}
	}

	return &BipartiteGraph{left, right, edges}, nil
}
