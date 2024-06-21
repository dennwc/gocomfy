package main

import (
	"context"
	"io"
	"math/rand"
	"os"

	"github.com/urfave/cli/v3"

	api "github.com/dennwc/gocomfy/graph/apinodes"
)

func init() {
	var (
		modelName   = "v1-5-pruned-emaonly.ckpt"
		seed        int64
		steps       = int64(20)
		cfg         = float64(8)
		promptStr   = "beautiful scenery nature glass bottle landscape, , purple galaxy bottle,"
		negativeStr = "text, watermark"
		width       = int64(512)
		height      = int64(512)
		sampler     = "euler"
		outFile     = "output.png"
	)
	Root.Commands = append(Root.Commands, &cli.Command{
		Name:  "t2i",
		Usage: "generate an image from a text prompt",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "model",
				Usage:       "models name (relative to comfy models dir)",
				Value:       modelName,
				Destination: &modelName,
			},
			&cli.IntFlag{
				Name:        "seed",
				Usage:       "seed for noise generation (0 to pick random)",
				Value:       seed,
				Destination: &seed,
			},
			&cli.IntFlag{
				Name:        "steps",
				Usage:       "sampler steps",
				Value:       steps,
				Destination: &steps,
			},
			&cli.FloatFlag{
				Name:        "cfg",
				Usage:       "cfg scale",
				Value:       cfg,
				Destination: &cfg,
			},
			&cli.StringFlag{
				Name:        "prompt",
				Usage:       "positive prompt text",
				Value:       promptStr,
				Destination: &promptStr,
			},
			&cli.StringFlag{
				Name:        "negative",
				Usage:       "negative prompt text",
				Value:       negativeStr,
				Destination: &negativeStr,
			},
			&cli.IntFlag{
				Name:        "width",
				Usage:       "image width",
				Value:       width,
				Destination: &width,
			},
			&cli.IntFlag{
				Name:        "height",
				Usage:       "image height",
				Value:       height,
				Destination: &height,
			},
			&cli.StringFlag{
				Name:        "sampler",
				Usage:       "sampler name",
				Value:       sampler,
				Destination: &sampler,
			},
			&cli.StringFlag{
				Name:        "out",
				Usage:       "output file name",
				Value:       outFile,
				Destination: &outFile,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			c, err := getClient(ctx)
			if err != nil {
				return err
			}
			defer c.Close()

			if seed == 0 {
				seed = rand.Int63()
			}

			g := api.New()
			_, model, clip, vae := api.CheckpointLoaderSimple(g, modelName)
			_, positive := api.CLIPTextEncode(g, clip, promptStr)
			_, negative := api.CLIPTextEncode(g, clip, negativeStr)
			_, latent := api.EmptyLatentImage(g, int(width), int(height), 1)
			_, outLatent := api.KSampler(g,
				model, positive, negative, latent,
				int(seed), int(steps), cfg,
				sampler, "normal", 1,
			)
			_, outImg := api.VAEDecode(g, outLatent, vae)
			out := api.PreviewImage(g, outImg)

			res, err := c.RunPrompt(ctx, g)
			if err != nil {
				return err
			}
			outRes := res[out.ID]

			rc, err := c.GetImageFile(ctx, outRes.Images[0])
			if err != nil {
				return err
			}
			defer rc.Close()

			f, err := os.Create(outFile)
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
			return f.Close()
		},
	})
}
