{{- define "egress" -}}
policyTypes:
- Egress
egress:
- to:
  - ipBlock:
    # Allow all except metadata-service
      cidr: 0.0.0.0/0
      except:
      - 169.254.169.254/32
{{- end}}