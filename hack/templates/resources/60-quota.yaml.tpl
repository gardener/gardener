<%
  import os, yaml

  values={}
  if context.get("values", "") != "":
    values=yaml.load(open(context.get("values", "")))

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
%># Quota object limiting the resources consumed by Shoot clusters either per cloud provider secret or per project/namespace.
---
apiVersion: garden.sapcloud.io/v1beta1
kind: Quota
metadata:
  name: ${value("metadata.name", "trial-quota")}
  namespace: ${value("metadata.namespace", "garden-trial")}<% annotations = value("metadata.annotations", {}); labels = value("metadata.labels", {}) %>
  % if annotations != {}:
  annotations: ${yaml.dump(annotations, width=10000)}
  % endif
  % if labels != {}:
  labels: ${yaml.dump(labels, width=10000)}
  % endif
spec:
  scope: ${value("spec.scope", "secret")}<% clusterLifetimeDays = value("spec.clusterLifetimeDays", "") %>
  % if clusterLifetimeDays != "":
  clusterLifetimeDays: ${clusterLifetimeDays}
  % else:
# clusterLifetimeDays: 14
  % endif
  metrics:<% metrics=value("spec.metrics", {}) %>
  % if metrics != {}:
  ${yaml.dump(metrics, width=10000)}
  % else:
    cpu: "200"
    gpu: "20"
    memory: 4000Gi
    storage.basic: 8000Gi
    storage.standard: 8000Gi
    storage.premium: 2000Gi
    loadbalancer: "100"
  % endif
