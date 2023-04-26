package filter

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/params"
)

// Grep defines various parameters for the grep plugin
type Grep struct {
	Regexps  []*Regexp  `json:"regexp,omitempty"`
	Excludes []*Exclude `json:"exclude,omitempty"`
	Ands     []*And     `json:"and,omitempty"`
	Ors      []*Or      `json:"or,omitempty"`
}

// Regexp defines the parameters for the regexp plugin
type Regexp struct {
	Key     *string `json:"key,omitempty"`
	Pattern *string `json:"pattern,omitempty"`
}

// Exclude defines the parameters for the exclude plugin
type Exclude struct {
	Key     *string `json:"key,omitempty"`
	Pattern *string `json:"pattern,omitempty"`
}

// And defines the parameters for the "and" plugin
type And struct {
	*Regexp  `json:"regexp,omitempty"`
	*Exclude `json:"exclude,omitempty"`
}

// Or defines the parameters for the "or" plugin
type Or struct {
	*Regexp  `json:"regexp,omitempty"`
	*Exclude `json:"exclude,omitempty"`
}

func (r *Regexp) Name() string {
	return "regexp"
}

func (e *Exclude) Name() string {
	return "exclude"
}

func (r *Regexp) Params(_ plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore(r.Name())
	ps.InsertPairs("key", fmt.Sprint(*r.Key))
	ps.InsertPairs("pattern", fmt.Sprint(*r.Pattern))
	return ps, nil
}

func (e *Exclude) Params(_ plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore(e.Name())
	ps.InsertPairs("key", fmt.Sprint(*e.Key))
	ps.InsertPairs("pattern", fmt.Sprint(*e.Pattern))
	return ps, nil
}
