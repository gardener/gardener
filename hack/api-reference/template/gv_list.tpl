{{- define "gvList" -}}
{{- $groupVersions := . -}}
<p>Packages:</p>
<ul>
{{- range $groupVersions }}
<li>
<a href="#{{ .GroupVersionString | replace "/" "%2f" }}">{{ .GroupVersionString }}</a>
</li>
{{- end }}
</ul>
{{ range $groupVersions }}
{{ template "gvDetails" . }}
{{ end }}
{{- end -}}
