package apigraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shoenig/test/must"
)

const testData = "../../testdata"

func TestJSON(t *testing.T) {
	for _, c := range []struct {
		name string
	}{
		{name: "default_api"},
	} {
		t.Run(c.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(testData, c.name+".json"))
			must.NoError(t, err)
			g, err := Unmarshal(data)
			must.NoError(t, err)

			got, err := Marshal(g)
			must.NoError(t, err)

			must.EqJSON(t, string(data), string(got))
		})
	}
}
