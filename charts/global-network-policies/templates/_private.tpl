{{- define "global-network-policies.except-networks" -}}
{{- range $i, $network := $ }}
    - ipBlock:
        cidr: {{ required ".network.cidr is required" $network.network }}
{{- if $network.except }}
        except:
{{ toYaml $network.except | indent 8 }}
{{- end }}
{{- end }}
{{- end -}}
