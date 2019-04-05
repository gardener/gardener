<%
  import os, yaml

  values={}
  if context.get("values", "") != "":
    values=yaml.load(open(context.get("values", "")), Loader=yaml.Loader)

  def value(path, default):
    keys=str.split(path, ".")
    root=values
    for key in keys:
      if isinstance(root, dict):
        if key in root:
          root=root[key]
        else:
          return default
      else:
        return default
    return root

  annotations = value("metadata.annotations", {}); labels = value("metadata.labels", {})
%># Plant cluster registration manifest through which an external kubernetes cluster will be mapped
---
apiVersion: core.gardener.cloud/v1alpha1
kind: Plant
metadata:
  name:  ${value("metadata.name", "example-plant")}<% annotations = value("metadata.annotations", {}); labels = value("metadata.labels", {}) %>
  namespace: ${value("metadata.namespace", "garden-dev")}
  % if annotations != {}:
  annotations: ${yaml.dump(annotations, width=1000, default_flow_style=None)}
  % endif
  % if labels != {}:
  labels: ${yaml.dump(labels, width=10000, default_flow_style=None)}
  % endif
spec:
  secretRef:
    name: my-external-cluster-secret
  endpoints:
  - name: Kibana Dashboard
    url: https://...
    purpose: logging
  - name: Prometheus Dashboard
    url: https://...
    purpose: monitoring