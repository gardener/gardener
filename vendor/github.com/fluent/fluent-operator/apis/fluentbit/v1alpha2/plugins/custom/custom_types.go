package custom

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// CustomPlugin is used to support filter plugins that are not implemented yet
type CustomPlugin struct {
	Config string `json:"config,omitempty"`
}

func (c *CustomPlugin) Name() string {
	return ""
}

func (a *CustomPlugin) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	kvs.Content = indentation(a.Config)
	return kvs, nil
}

func indentation(str string) string {
	splits := strings.Split(str, "\n")
	var buf bytes.Buffer
	for _, i := range splits {
		if i != "" {
			buf.WriteString(fmt.Sprintf("    %s\n", strings.TrimSpace(i)))
		}
	}
	return buf.String()
}
