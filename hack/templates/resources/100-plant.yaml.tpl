<%
  import os, yaml

  values={}
  if context.get("values", "") != "":
    values=yaml.load(open(context.get("values", "")))

  if context.get("cloud", "") == "":
    raise Exception("missing --var cloud={aws,azure,gcp,alicloud,openstack,local} flag")

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
  name: ${value("metadata.name", "johndoe-" + cloud)}
  namespace: ${value("metadata.namespace", "garden-dev")}
  % if annotations != {}:
  annotations: ${yaml.dump(annotations, width=1000)}
  % endif
  % if labels != {}:
  labels: ${yaml.dump(labels, width=10000)}
  % endif
spec:
  secretRef:
    name: my-external-cluster
    namespace: my-namespace
  monitoring:
    endpoints:
    - name: Kubernetes Dashboard
      url: https://...
    - name: Prometheus
      url: https://...
  logging:
    endpoints:
    - name: Kibana
      url: https://...