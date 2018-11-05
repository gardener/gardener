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
%>---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${value("metadata.name", "auditpolicy")}
  namespace: ${value("metadata.namespace", "garden-dev")}<% annotations = value("metadata.annotations", {}); labels = value("metadata.labels", {}) %>
  % if annotations != {}:
  annotations: ${yaml.dump(annotations, width=10000)}
  % endif
  % if labels != {}:
  labels: ${yaml.dump(labels, width=10000)}
  % endif
data:
  policy: |-<% policy=value("data.policy", "") %>
    % if policy != "":
    ${yaml.dump(policy, width=10000)}
    % else:
    apiVersion: audit.k8s.io/v1beta1
    kind: Policy
    rules:
      - level: Metadata
        omitStages:
          - "RequestReceived"
    % endif