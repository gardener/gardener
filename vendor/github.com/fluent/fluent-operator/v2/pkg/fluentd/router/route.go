package router

import (
	"crypto/md5"
	"fmt"
	"io"
	"sort"
)

type RouteMatch struct {
	// Label definition to match record. Example: app:nginx
	Labels map[string]string `json:"labels,omitempty"`
	// Comma separated list of namespaces. Ignored if left empty.
	Namespaces []string `json:"namespaces,omitempty"`
	// Comma separated list of hosts. Ignored if left empty.
	Hosts []string `json:"hosts,omitempty"`
	// Comma separated list of container names. Ignored if left empty.
	ContainerNames []string `json:"containerNames,omitempty"`
	// Negate the selector meaning to exclude matches
	Negate *bool `json:"negate,omitempty"`
}

type Route struct {
	Id string `json:"id,omitempty"`
	// Route the matching record to the given label
	Label *string `json:"label,omitempty"`
	// // Tag the matching record to the given tag
	Tag *string `json:"tag,omitempty"`
	// List of match statements. Repeatable.
	RouteMatches []*RouteMatch `json:"routeMatches,omitempty"`
}

// NewRoute will new a route witch the given label, tag , and matches
func NewRoute(id, namespace, name string, matches []*RouteMatch) (*Route, error) {
	route := &Route{
		Id:           id,
		RouteMatches: matches,
	}
	err := route.calculateRouteLabel(namespace, name)
	return route, err
}

// calculateRouteLabel will calculate the router label/tag of the given cfgconfig or clustercfgconfig
func (r *Route) calculateRouteLabel(namespace, name string) error {
	b := md5.New()
	_, err := io.WriteString(b, namespace)
	if err != nil {
		return err
	}
	_, err = io.WriteString(b, name)
	if err != nil {
		return err
	}
	for _, match := range r.RouteMatches {
		// sort namespaces
		if len(match.Namespaces) > 0 {
			sort.Strings(match.Namespaces)
			for _, n := range match.Namespaces {
				if _, err := io.WriteString(b, n); err != nil {
					return err
				}
			}
		}

		// sort labels
		if len(match.Labels) > 0 {
			keys := make([]string, 0, len(match.Labels))
			for k := range match.Labels {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if _, err := io.WriteString(b, k); err != nil {
					return err
				}
				if _, err := io.WriteString(b, match.Labels[k]); err != nil {
					return err
				}
			}
		}
	}

	routeLabel := fmt.Sprintf("@%x", b.Sum(nil))
	r.Label = &routeLabel
	r.Tag = &routeLabel
	return nil
}
