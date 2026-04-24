package clone

import (
	"gopkg.in/yaml.v3"

	"github.com/LocalKinAI/kinclaw/pkg/soul"
)

// marshalMeta renders a Meta back to YAML. It's a thin wrapper
// around yaml.v3 with indent=2; factored out so Clone's renderSoul
// stays readable.
func marshalMeta(meta soul.Meta) ([]byte, error) {
	return yaml.Marshal(meta)
}
