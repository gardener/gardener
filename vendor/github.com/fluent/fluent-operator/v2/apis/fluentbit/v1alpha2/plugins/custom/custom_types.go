package custom

import (
	"bytes"
	"fmt"
	"github.com/fluent/fluent-operator/v2/pkg/utils"
	"strings"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// CustomPlugin is used to support filter plugins that are not implemented yet. <br />
// **For example usage, refer to https://github.com/jjsiv/fluent-operator/blob/master/docs/best-practice/custom-plugin.md**
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

func MakeCustomConfigNamespaced(customConfig string, namespace string) string {
	var buf bytes.Buffer
	sections := strings.Split(customConfig, "\n")
	for _, section := range sections {
		section = strings.TrimSpace(section)
		idx := strings.LastIndex(section, " ")
		if strings.HasPrefix(section, "Match_Regex") {
			buf.WriteString(fmt.Sprintf("Match_Regex %s\n", utils.GenerateNamespacedMatchRegExpr(namespace, section[idx+1:])))
			continue
		}
		if strings.HasPrefix(section, "Match") {
			buf.WriteString(fmt.Sprintf("Match %s\n", utils.GenerateNamespacedMatchExpr(namespace, section[idx+1:])))
			continue
		}
		buf.WriteString(fmt.Sprintf("%s\n", section))
	}
	return buf.String()
}
