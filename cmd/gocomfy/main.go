package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/urfave/cli/v3"

	"github.com/dennwc/gocomfy"
)

var (
	Root = &cli.Command{
		Name:  "gocomfy",
		Usage: "Command line client for ComfyUI",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "addr",
				Usage:       "host and port for ComfyUI",
				Value:       comfyHost,
				Destination: &comfyHost,
				Persistent:  true,
			},
		},
	}
	comfyHost = "localhost:8188"
)

func getClient(ctx context.Context) (*gocomfy.Client, error) {
	return gocomfy.NewClient(ctx, comfyHost)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := Root.Run(ctx, os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
