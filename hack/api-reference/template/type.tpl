{{- define "type" -}}
{{- $args := . -}}
{{- $gv := index $args 0 -}}
{{- $type := index $args 1 -}}
<h3 id="{{ $type.Name | lower }}">{{ $type.Name }}
</h3>
{{ if $type.IsAlias }}
{{- $underlyingStr := markdownRenderType $type.UnderlyingType -}}
<p><em>Underlying type: {{ regexReplaceAll `\[([^\]]+)\]\(([^)]+)\)` $underlyingStr `<a href="${2}">${1}</a>` }}</em></p>
{{ end }}
{{ if $type.References }}
<p>
(<em>Appears on:</em>
{{- $first := true -}}
{{- range $type.SortedReferences -}}
{{- if not $first -}}, {{ end -}}
{{- $first = false -}}
<a href="#{{ .Name | lower }}">{{ .Name }}</a>
{{- end -}}
)
</p>
{{ end }}
<p>
{{ $type.Doc }}
</p>
{{ if $type.Members }}
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
{{ if $type.GVK }}
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>{{ $type.GVK.Group }}/{{ $type.GVK.Version }}</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>{{ $type.GVK.Kind }}</code></td>
</tr>
{{ end }}
{{ range $type.Members }}
{{- $typeStr := markdownRenderType .Type -}}
<tr>
<td>
<code>{{ .Name }}</code></br>
<em>
{{ regexReplaceAll `\[([^\]]+)\]\(([^)]+)\)` $typeStr `<a href="${2}">${1}</a>` }}
</em>
</td>
<td>
{{ template "type_members" . }}
</td>
</tr>
{{ end }}
</tbody>
</table>
{{ end }}
{{- end -}}
