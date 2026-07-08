# ComfyUI client for Go

This package provides API client for [ComfyUI](https://github.com/Comfy-Org/ComfyUI).

**Features:**
- Run arbitrary ComfyUI prompts.
- Type-safe graph constructors for builtin ComfyUI node types.
- Generate code directly from workflow files in API format.
- Image upload and download. Handles cached images.
- Prompt execution events.

## Usage

Any ComfyUI workflow can be exported in API format (`File -> Export (API)`) and converted to code:

```shell
gocomfy code-from-api -i ./testdata/default_api.json
```

Generates the following Go code:

```go
package workflow

import (
	"github.com/dennwc/gocomfy/graph/apinodes"
)

func Workflow() *apinodes.Graph {
	g := apinodes.New()
	_, n4model, n4clip, n4vae := apinodes.CheckpointLoaderSimple(g, "some/model.safetensors")
	_, n6conditioning := apinodes.CLIPTextEncode(g, n4clip, "beautiful scenery nature glass bottle landscape, , purple galaxy bottle,")
	_, n7conditioning := apinodes.CLIPTextEncode(g, n4clip, "text, watermark")
	_, n5latent := apinodes.EmptyLatentImage(g, 512, 512, 1)
	_, n3latent := apinodes.KSampler(g, n4model, n6conditioning, n7conditioning, n5latent, 156680208700286, 20, 8, "euler", "normal", 1)
	_, n8image := apinodes.VAEDecode(g, n3latent, n4vae)
	apinodes.SaveImage(g, n8image, "ComfyUI")
	return g
}
```

See [cmd/gocomfy/text_to_image.go](./cmd/gocomfy/text_to_image.go) for more examples.

# License

MIT