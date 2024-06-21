package apinodes

import "github.com/dennwc/gocomfy/graph/apigraph"

func New() *Graph {
	return apigraph.New()
}

type Graph = apigraph.Graph
type Node = apigraph.Node
type Link = apigraph.Link
type Value = apigraph.Value
type Int = apigraph.Int
type Float = apigraph.Float
type String = apigraph.String
type Bool = apigraph.Bool
