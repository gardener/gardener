{{- define "egress" -}}
policyTypes:
- Egress
egress:
- to:
  - ipBlock:
    # Allow all except metadata-service
      cidr: 0.0.0.0/0
      except:
      {{ if eq .Values.cloudProvider "alicloud" }}
      - 100.100.100.200/32
      {{ else }}
      - 169.254.169.254/32
      {{ end }}
{{- end}}