package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ory/dockertest/v3"
	"golang.org/x/exp/maps"

	"github.com/dennwc/gocomfy"
	"github.com/dennwc/gocomfy/graph/classes"
	"github.com/dennwc/gocomfy/graph/types"
)

var (
	forceUpdate = flag.Bool("f", false, "force update the classes")
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	flag.Parse()
	if err := run(ctx); err != nil {
		slog.Error("failed", "err", err)
		os.Exit(1)
	}
}

func getClasses(ctx context.Context) ([]byte, error) {
	const path = "./testdata/object_info.json"
	if !*forceUpdate {
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	slog.Info("connecting to Docker")
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, fmt.Errorf("could not construct pool: %w", err)
	}
	if err = pool.Client.Ping(); err != nil {
		return nil, fmt.Errorf("could not connect to Docker: %w", err)
	}
	slog.Info("starting ComfyUI")
	cont, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:       "comfyui-tmp",
		Repository: "ghcr.io/oxc/comfyui", Tag: "latest-cpu",
		ExposedPorts: []string{"8188/tcp"},
	})
	if err != nil {
		return nil, fmt.Errorf("could not start ComfyUI: %w", err)
	}
	defer pool.Purge(cont)
	addr := cont.GetHostPort("8188/tcp")
	if addr == "" {
		return nil, fmt.Errorf("no address for container")
	}
	slog.Info("connecting to ComfyUI", "addr", addr)
	if err = waitTCPPort(pool, addr); err != nil {
		return nil, err
	}
	time.Sleep(5 * time.Second)
	cli, err := gocomfy.NewClient(ctx, addr, gocomfy.WithoutWebsocket())
	if err != nil {
		return nil, fmt.Errorf("could not connect to ComfyUI: %w", err)
	}
	defer cli.Close()

	slog.Info("loading objects")
	rc, err := cli.ObjectsInfoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get object info: %w", err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	_ = rc.Close()
	_ = pool.Purge(cont)
	if err != nil {
		return nil, fmt.Errorf("could not read object info: %w", err)
	}
	var buf bytes.Buffer
	err = json.Indent(&buf, raw, "", "\t")
	if err != nil {
		return nil, fmt.Errorf("could not parse object info: %w", err)
	}
	err = os.WriteFile(path, buf.Bytes(), 0644)
	if err != nil {
		return nil, fmt.Errorf("could not write object info: %w", err)
	}
	return raw, nil
}

func run(ctx context.Context) error {
	raw, err := getClasses(ctx)
	if err != nil {
		return err
	}
	classByName, err := classes.Decode(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("could not decode classes: %w", err)
	}
	slog.Info("generating files...")
	if err = generate(classByName); err != nil {
		return err
	}
	slog.Info("done!")
	return nil
}

func waitTCPPort(pool *dockertest.Pool, addr string) error {
	prev := pool.MaxWait
	defer func() {
		pool.MaxWait = prev
	}()
	pool.MaxWait = 10 * time.Second
	if err := pool.Retry(func() error {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			slog.Info("waiting for port to become available", "err", err)
			return err
		}
		_ = conn.Close()
		return nil
	}); err != nil {
		return fmt.Errorf("cannot connect to %s: %w", addr, err)
	}
	return nil
}

const generatedHeader = "// Code generated by comfy-classes. DO NOT EDIT.\n\n"

func generate(classByName classes.Classes) error {
	var (
		anodes bytes.Buffer
	)
	anodes.WriteString(generatedHeader)
	anodes.WriteString(`package apinodes

`)
	names := maps.Keys(classByName)
	slices.Sort(names)
	generateGraphLinks(&anodes, classByName)
	for _, name := range names {
		c := classByName[name]
		generateGraphNodes(&anodes, c)
	}
	if err := os.WriteFile("./graph/apinodes/classes.go", anodes.Bytes(), 0644); err != nil {
		return err
	}
	return nil
}

func exportedName(name string) string {
	r := rune(name[0])
	if unicode.IsUpper(r) {
		return name
	}
	return string(unicode.ToUpper(r)) + name[1:]
}

var nameReplacer = strings.NewReplacer(
	".", "_",
)

func argName(name string) string {
	name = strings.ToLower(name)
	switch name {
	case "type":
		return "typ"
	}
	name = nameReplacer.Replace(name)
	return name
}

func linkType(name string) string {
	return name
}

func generateGraphLinks(buf *bytes.Buffer, all map[types.NodeClass]*classes.Class) {
	types := make(map[types.TypeName]struct{})
	for _, c := range all {
		for _, p := range c.Inputs {
			if p.Kind == classes.InputHidden || p.Type.IsScalar() {
				continue
			}
			types[p.Type] = struct{}{}
		}
		for _, p := range c.Outputs {
			if p.Type.IsScalar() {
				continue
			}
			types[p.Type] = struct{}{}
		}
	}
	names := maps.Keys(types)
	slices.Sort(names)
	for _, name := range names {
		buf.WriteString("type ")
		buf.WriteString(linkType(string(name)))
		buf.WriteString(" Link\n")
	}
	buf.WriteString("\n")
}

func generateGraphNodes(buf *bytes.Buffer, c *classes.Class) {
	cname := exportedName(string(c.Name))
	if c.Title != "" && c.Title != cname {
		buf.WriteString("// ")
		buf.WriteString(cname)
		buf.WriteString(" - ")
		buf.WriteString(c.Title)
		buf.WriteString("\n")
	}
	buf.WriteString("func ")
	buf.WriteString(cname)
	// arguments
	argNames := make(map[string]struct{})
	buf.WriteString("(g *Graph")
	// input links
	for _, p := range c.Inputs {
		if p.Kind == classes.InputHidden || p.Type.IsScalar() {
			continue
		}
		name := argName(p.Name)
		argNames[name] = struct{}{}
		buf.WriteString(", ")
		buf.WriteString(name)
		buf.WriteString(" ")
		buf.WriteString(linkType(string(p.Type)))
	}
	// input scalars
	for i, p := range c.Inputs {
		if p.Kind == classes.InputHidden || !p.Type.IsScalar() {
			continue
		}
		name := argName(p.Name)
		argNames[name] = struct{}{}
		buf.WriteString(", ")
		buf.WriteString(name)
		if i+1 < len(c.Inputs) {
			p2 := c.Inputs[i+1]
			if p.Type == p2.Type && p2.Kind != classes.InputHidden {
				continue
			}
		}
		buf.WriteString(" ")
		switch p.Type {
		case types.IntType:
			buf.WriteString("int")
		case types.FloatType:
			buf.WriteString("float64")
		case types.StringType:
			buf.WriteString("string")
		case types.BoolType:
			buf.WriteString("bool")
		default:
			panic("unsupported scalar: " + string(p.Type))
		}
	}
	buf.WriteString(") ")
	// returns
	buf.WriteString("(_ *Node")
	for _, p := range c.Outputs {
		buf.WriteString(", ")
		name := argName(p.Name)
		if _, ok := argNames[name]; ok {
			name = "out_" + name
		}
		buf.WriteString(name)
		buf.WriteString(" ")
		buf.WriteString(linkType(string(p.Type)))
	}
	buf.WriteString(") ")
	// body
	buf.WriteString("{\n")
	defer buf.WriteString("}\n\n")

	fmt.Fprintf(buf, `	n := &Node{
		Class: %q,
		Inputs: map[string]Value{
`, c.Name)
	for _, p := range c.Inputs {
		if p.Kind != classes.InputRequired {
			continue
		}
		buf.WriteString("\t\t\t")
		buf.WriteString(strconv.Quote(p.Name))
		buf.WriteString(": ")
		switch p.Type {
		case types.IntType:
			buf.WriteString("Int")
		case types.FloatType:
			buf.WriteString("Float")
		case types.StringType:
			buf.WriteString("String")
		case types.BoolType:
			buf.WriteString("Bool")
		default:
			buf.WriteString("Link")
		}
		buf.WriteString("(")
		buf.WriteString(argName(p.Name))
		buf.WriteString(")")
		buf.WriteString(",\n")
	}
	buf.WriteString("\t\t},\n\t}\n")
	if len(c.Outputs) == 0 {
		buf.WriteString("\tg.Add(n)\n")
		buf.WriteString("\treturn n")
		return
	}
	buf.WriteString("\tid := g.Add(n)\n")
	buf.WriteString("\treturn n")
	for i, p := range c.Outputs {
		buf.WriteString(", ")
		buf.WriteString(linkType(string(p.Type)))
		fmt.Fprintf(buf, "{NodeID: id, OutPort: %d}", i)
	}
	buf.WriteString("\n")
}