package gocomfy

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"os"
	"testing"
	"time"

	"github.com/shoenig/test/must"

	"github.com/dennwc/gocomfy/graph/apigraph"
)

func testLogger(t testing.TB) *slog.Logger {
	lvl := slog.LevelInfo
	if os.Getenv("DEBUG") != "" {
		lvl = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}

func testClient(t testing.TB) *Client {
	const env = "COMFY_HOST"
	host := os.Getenv(env)
	if host == "" {
		t.Skipf("%s not set", env)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	c, err := NewClient(ctx, host, WithLog(testLogger(t)), WithOnQueueSize(func(queue int) {
		t.Logf("queue: %d", queue)
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

func TestPrompt(t *testing.T) {
	const env = "COMFY_MODEL"
	model := os.Getenv(env)
	if model == "" {
		t.Skipf("%s not set", env)
	}

	g, err := apigraph.ReadFile("./testdata/default_api.json")
	must.NoError(t, err)

	// set model
	loader := g.Nodes[4]
	must.EqOp(t, loader.Class, "CheckpointLoaderSimple")
	loader.Inputs["ckpt_name"] = apigraph.String(model)

	// get sampler for seed randomization
	sampler := g.Nodes[3]
	must.EqOp(t, sampler.Class, "KSampler")

	// get SaveImage to swap it later
	save := g.Nodes[9]
	must.EqOp(t, save.Class, "SaveImage")
	delete(save.Inputs, "filename_prefix")

	c := testClient(t)

	runTest := func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		p, err := c.Prompt(ctx, g)
		must.NoError(t, err)
		var (
			start     int
			done      int
			nodeStart int
			nodeDone  int
			images    []ImageRef
		)
		for ev := range p.Events() {
			switch ev := ev.(type) {
			case ExecStart:
				start++
				t.Log("prompt started")
			case ExecDone:
				done++
				t.Log("prompt complete!")
			case NodeStart:
				nodeStart++
				t.Logf("node %v: started", int(ev.Node))
			case ExecCache:
				nodeStart += len(ev.Nodes)
				nodeDone += len(ev.Nodes)
				t.Logf("cached nodes: %v", len(ev.Nodes))
			case NodeProg:
				t.Logf("node %v: progress=%d/%d", int(ev.Node), ev.Value, ev.Max)
			case NodeDone:
				nodeDone++
				t.Logf("node %v: done", int(ev.Node))
				images = append(images, ev.Images...)
				for _, img := range ev.Images {
					t.Logf("node %v: image=%q", int(ev.Node), img.Filename)
				}
			default:
				t.Logf("%#v", ev)
			}
		}
		must.EqOp(t, 1, start)
		must.EqOp(t, 1, done)
		must.EqOp(t, len(g.Nodes), nodeDone)
		must.EqOp(t, nodeStart, nodeDone)
		must.EqOp(t, 1, len(images))
	}

	t.Run("PreviewImage", func(t *testing.T) {
		sampler.Inputs["seed"] = apigraph.Int(rand.Uint32())
		save.Class = "PreviewImage"
		runTest(t)
	})
}
