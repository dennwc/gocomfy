package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"

	"github.com/dennwc/gocomfy/graph/apigraph"
	"github.com/dennwc/gocomfy/graph/apinodes"
	"github.com/dennwc/gocomfy/graph/classes"
	"github.com/dennwc/gocomfy/graph/types"
	cli "github.com/urfave/cli/v3"
)

func init() {
	var flags struct {
		Pkg   string
		Input string
	}
	cmd := &cli.Command{
		Name:  "code-from-api",
		Usage: "Generate Go code from workflow API JSON",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "input",
				Aliases:     []string{"i"},
				Usage:       "Input JSON file (API format)",
				Destination: &flags.Input,
			},
			&cli.StringFlag{
				Name:        "package",
				Aliases:     []string{"pkg"},
				Usage:       "Go package name to use",
				Destination: &flags.Pkg,
			},
		},
	}
	Root.Commands = append(Root.Commands, cmd)
	cmd.Action = func(ctx context.Context, cmd *cli.Command) error {
		return codeFromAPI(os.Stdout, flags.Pkg, flags.Input)
	}
}

type nodeUsage struct {
	Out      int
	OutUse   []int
	OutNames map[int]string
}

func codeFromAPI(w io.Writer, pkg string, inputPath string) error {
	if pkg == "" {
		pkg = "workflow"
	}
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	f, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	g, err := apigraph.Read(f)
	if err != nil {
		return err
	}
	usage := make(map[types.NodeID]*nodeUsage)
	for _, n := range g.Nodes {
		u := usage[n.ID]
		if u == nil {
			u = new(nodeUsage)
			usage[n.ID] = u
		}
		for _, v := range n.Inputs {
			if l, ok := v.(apigraph.Link); ok {
				u2 := usage[l.NodeID]
				if u2 == nil {
					u2 = new(nodeUsage)
					usage[l.NodeID] = u2
				}
				u2.Out++
				if outs := u2.OutUse; l.OutPort >= len(outs) {
					u2.OutUse = make([]int, l.OutPort+1)
					copy(u2.OutUse, outs)
				}
				u2.OutUse[l.OutPort]++
			}
		}
	}
	var roots []types.NodeID
	for id, u := range usage {
		if u.Out == 0 {
			roots = append(roots, id)
		}
	}
	slices.Sort(roots)
	seen := make(map[types.NodeID]struct{})
	bw.WriteString("package ")
	bw.WriteString(pkg)
	bw.WriteString("\n")
	bw.WriteString(`
import (
	"github.com/dennwc/gocomfy/graph/apinodes"
)

func Workflow() *apinodes.Graph {
	g := apinodes.New()
`)
	defer bw.WriteString("\treturn g\n}\n")
	for _, root := range roots {
		n := g.Nodes[root]
		if n == nil {
			continue
		}
		codeFromAPINode(bw, g, n, usage, seen)
	}
	return nil
}

func codeFromAPINode(w *bufio.Writer, g *apigraph.Graph, n *apigraph.Node, usage map[types.NodeID]*nodeUsage, seen map[types.NodeID]struct{}) {
	if _, ok := seen[n.ID]; ok {
		return
	}
	seen[n.ID] = struct{}{}
	u := usage[n.ID]
	c := apinodes.ClassByName[n.Class]
	if c == nil {
		c = &classes.Class{}
		fields := slices.Collect(maps.Keys(n.Inputs))
		slices.Sort(fields)
		for _, field := range fields {
			c.Inputs = append(c.Inputs, classes.Input{Name: field})
		}
		for range u.OutUse {
			c.Outputs = append(c.Outputs, classes.Output{})
		}
	}
	for _, p := range c.Inputs {
		if p.Kind == classes.InputHidden {
			continue
		}
		v, ok := n.Inputs[p.Name].(apigraph.Link)
		if !ok {
			continue
		}
		n2 := g.Nodes[v.NodeID]
		if n2 == nil {
			continue
		}
		codeFromAPINode(w, g, n2, usage, seen)
	}
	w.WriteString("\t")
	if len(c.Outputs) != 0 && u.Out != 0 {
		w.WriteString("_")
		for i, p := range c.Outputs {
			used := false
			if i < len(u.OutUse) {
				used = u.OutUse[i] != 0
			}
			w.WriteString(", ")
			if used {
				name := "_" + strconv.Itoa(i)
				if p.Name != "" {
					name = classes.GoArgName(p.Name)
				}
				name = fmt.Sprintf("n%s%s", n.ID.String(), name)
				if u.OutNames == nil {
					u.OutNames = make(map[int]string)
				}
				u.OutNames[i] = name
				w.WriteString(name)
			} else {
				w.WriteString("_")
			}
		}
		w.WriteString(" := ")
	}
	w.WriteString("apinodes.")
	w.WriteString(classes.GoNodeType(string(n.Class)))
	w.WriteString("(g")
	defer w.WriteString(")\n")
	input := func(p *classes.Input) {
		w.WriteString(", ")
		v := n.Inputs[p.Name]
		switch v := v.(type) {
		case nil:
			if !p.Type.IsScalar() {
				w.WriteString("apinodes.")
				w.WriteString(classes.GoLinkType(string(p.Type)))
				w.WriteString("{}")
			} else {
				w.WriteString("nil")
			}
		case apigraph.Link:
			u2 := usage[v.NodeID]
			if u2 == nil {
				u2 = new(nodeUsage)
			}
			name := u2.OutNames[v.OutPort]
			if name == "" {
				name = fmt.Sprintf("n%s_%d", v.NodeID.String(), v.OutPort)
			}
			w.WriteString(name)
		case apigraph.String:
			fmt.Fprintf(w, "%q", string(v))
		default:
			fmt.Fprintf(w, "%v", v)
		}
	}
	for _, p := range c.Inputs {
		if p.Kind == classes.InputHidden || p.Type.IsScalar() {
			continue
		}
		input(&p)
	}
	for _, p := range c.Inputs {
		if p.Kind == classes.InputHidden || !p.Type.IsScalar() {
			continue
		}
		input(&p)
	}
}
