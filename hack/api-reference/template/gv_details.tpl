{{- define "gvDetails" -}}
{{- $gv := . -}}
<h2 id="{{ $gv.GroupVersionString }}">{{ $gv.GroupVersionString }}</h2>
<p>
{{ $gv.Doc | regexReplaceAll "(?m)^SPDX-[^\n]*" "" | trim }}
</p>
{{- $rootTypes := list -}}
{{- range $gv.SortedTypes -}}
  {{- $hasMetadata := false -}}
  {{- range .Members }}{{ if eq .Name "metadata" }}{{- $hasMetadata = true -}}{{ end }}{{- end -}}
  {{- if $hasMetadata -}}
    {{- $rootTypes = append $rootTypes . -}}
  {{- end -}}
{{- end -}}
{{- if $rootTypes }}
Resource Types:
<ul>
{{- range $rootTypes }}
<li>
<a href="#{{ .Name | lower }}">{{ .Name }}</a>
</li>
{{- end }}
</ul>
{{- end }}
{{ range $gv.SortedTypes }}
{{ template "type" (list $gv .) }}
{{ end }}
{{- end -}}
