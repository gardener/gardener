// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	gonumgraph "gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

// DebugHandlerPath is the HTTP handler path for this debug handler.
const DebugHandlerPath = "/debug/resource-dependency-graph"

type handler struct {
	graph *simple.DirectedGraph
}

// NewDebugHandler creates a new HTTP handler for debugging the resource dependency graph.
func NewDebugHandler(graph *graph) http.HandlerFunc {
	return (&handler{graph.graph}).Handle
}

func (h *handler) Handle(w http.ResponseWriter, r *http.Request) {
	var (
		out string

		kind      = getQueryParameter(r.URL.Query(), "kind")
		namespace = getQueryParameter(r.URL.Query(), "namespace")
		name      = getQueryParameter(r.URL.Query(), "name")

		nodesIterator = h.graph.Nodes()
		nodes         []*vertex
	)

	// On large landscapes, if there are many vertices then the entire graph cannot be rendered fast enough. Hence, we
	// apply some default filtering for the 'seed' kind.
	if kind == "" && namespace == "" && name == "" && nodesIterator.Len() > 2000 {
		kind = "Seed"
	}

	// Filter for all relevant nodes and sort them.
	for nodesIterator.Next() {
		v := nodesIterator.Node().(*vertex)

		if (kind != "" && vertexTypes[v.vertexType] != kind) ||
			(namespace != "" && v.namespace != namespace) ||
			(name != "" && v.name != name) {
			continue
		}

		nodes = append(nodes, v)
	}
	sort.Sort(vertexSorter(nodes))

	// Render filtering form.
	out += `
<form action="` + DebugHandlerPath + `" method="GET">
  <select name="kind">`
	out += fmt.Sprintf(`<option value=""%s>&lt;all&gt;</option>`, selected("", kind))
	for _, vt := range vertexTypes {
		out += fmt.Sprintf(`<option value="%s"%s>%s</option>`, vt, selected(vt, kind), vt)
	}
	out += `
  </select>
  <input type="test" name="namespace" value="` + namespace + `" />
  <input type="test" name="name" value="` + name + `" />
  <input type="submit" value="go" />
</form>` + separate(true)

	// Iterate over each nodes, determine and sort their incoming and outgoing neighbors, and render the output.
	for i, v := range nodes {
		out += indent(0, "# %s", link(v))

		for _, n := range []struct {
			prefix   string
			iterator gonumgraph.Nodes
		}{
			{"<- (incoming)", h.graph.To(v.ID())},
			{"-> (outgoing)", h.graph.From(v.ID())},
		} {
			var neighbors []*vertex
			for n.iterator.Next() {
				neighbors = append(neighbors, n.iterator.Node().(*vertex))
			}
			sort.Sort(vertexSorter(neighbors))

			if len(neighbors) > 0 {
				out += indent(1, "%s (%d)", n.prefix, len(neighbors))
				for _, u := range neighbors {
					out += indent(2, link(u))
				}
			}
		}

		out += emptyNewline() + separate(i < len(nodes)-1)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<font size="2" face="Courier New">`+out+`</font>`)
}

func getQueryParameter(query url.Values, param string) string {
	var out string
	values, ok := query[param]
	if ok && len(values[0]) >= 1 {
		out = values[0]
	}
	return out
}

type vertexSorter []*vertex

func (v vertexSorter) Len() int { return len(v) }

func (v vertexSorter) Swap(i, j int) { v[i], v[j] = v[j], v[i] }

func (v vertexSorter) Less(i, j int) bool {
	if vertexTypes[v[i].vertexType] < vertexTypes[v[j].vertexType] {
		return true
	} else if vertexTypes[v[i].vertexType] > vertexTypes[v[j].vertexType] {
		return false
	}

	if v[i].namespace < v[j].namespace {
		return true
	} else if v[i].namespace > v[j].namespace {
		return false
	}

	return v[i].name < v[j].name
}

func emptyNewline() string {
	return "|<br />"
}

func separate(withNewBeginning bool) string {
	result := "-------------------------------------------------------------------------------<br />"
	if withNewBeginning {
		result += emptyNewline()
	}
	return result
}

func indent(level int, format string, a ...any) string {
	return fmt.Sprintf("| "+strings.Repeat("&nbsp;&nbsp;", level)+format+"<br />", a...)
}

func link(v *vertex) string {
	path := fmt.Sprintf("%s?kind=%s", DebugHandlerPath, vertexTypes[v.vertexType])
	out := fmt.Sprintf(`<a href="%s">%s</a>:`, path, vertexTypes[v.vertexType])

	if v.namespace != "" {
		path += "&namespace=" + v.namespace
		out += fmt.Sprintf(`<a href="%s">%s</a>/`, path, v.namespace)
	}

	path += "&name=" + v.name
	out += fmt.Sprintf(`<a href="%s">%s</a>`, path, v.name)

	return out
}

func selected(name, selected string) string {
	if selected == name {
		return " selected"
	}
	return ""
}
