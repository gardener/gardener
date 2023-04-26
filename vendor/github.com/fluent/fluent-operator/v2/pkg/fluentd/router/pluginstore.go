package router

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/params"
)

type RouterCommon struct {
	Type string `json:"type"`
	// The @label parameter is to route the input events to <label> sections
	Label string `json:"label,omitempty"`
}

type LabelRouter struct {
	*RouterCommon `json:"inline,omitempty"`
	// Emit mode. If batch, the plugin will emit events per labels matched. Enum: record, batch.
	// +kubebuilder:validation:Enum:=record;batch
	EmitMode *string `json:"emit_mode,omitempty"`
	// Sticky tags will match only one record from an event stream. The same tag will be treated the same way.
	StickyTags *string `json:"stickyTags,omitempty"`
	// If defined all non-matching record passes to this label.
	DefaultRoute *string `json:"defaultRoute,omitempty"`
	// If defined all non-matching record rewrited to this tag. (Can be used with label simoultanesly)
	DefaultTag *string `json:"defaultTag,omitempty"`
	// Route the log if match with parameters defined
	Routes []*Route `json:"routes,omitempty"`
}

// NewRoutePlugin will create a route pluginstore for each fluentd config instance
func (r *Route) NewRoutePlugin() (*params.PluginStore, error) {
	ps := params.NewPluginStore("route")
	childs := make([]*params.PluginStore, 0)

	if r.Label == nil {
		return ps, errors.New("label can not be empty")
	}

	ps.InsertPairs("@label", fmt.Sprint(*r.Label))
	// ps.InsertPairs("tag", fmt.Sprint(*r.Label))

	if len(r.RouteMatches) > 0 {
		for _, match := range r.RouteMatches {
			if match != nil {
				child := params.NewPluginStore("match")
				if len(match.Labels) > 0 {
					labels := make([]string, 0, len(match.Labels))
					keys := make([]string, 0, len(match.Labels))
					for k := range match.Labels {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						labels = append(labels, fmt.Sprintf("%s:%s", k, match.Labels[k]))
					}
					child.InsertPairs("labels", strings.Join(labels, ","))
				}
				if len(match.Namespaces) > 0 {
					sort.Strings(match.Namespaces)
					child.InsertPairs("namespaces", strings.Join(match.Namespaces, ","))
				}
				if len(match.Hosts) > 0 {
					sort.Strings(match.Hosts)
					child.InsertPairs("hosts", strings.Join(match.Hosts, ","))
				}
				if len(match.ContainerNames) > 0 {
					sort.Strings(match.ContainerNames)
					child.InsertPairs("container_names", strings.Join(match.ContainerNames, ","))
				}
				if match.Negate != nil {
					child.InsertPairs("negate", fmt.Sprint(*match.Negate))
				}

				childs = append(childs, child)
			}
		}
	}

	ps.InsertChilds(childs...)
	return ps, nil
}

// NewGlobalRouter will create a global router to store routes
func NewGlobalRouter(id string) *params.PluginStore {
	ps := params.NewPluginStore("match")
	ps.InsertPairs("@id", id)
	ps.InsertPairs("@type", "label_router")
	ps.InsertPairs("tag", "**")
	return ps
}
