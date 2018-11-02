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
%># Project objects logically group team members, secrets, and Shoot clusters. They result in Kubernetes namespaces
# prefixed with "garden-".
---
apiVersion: garden.sapcloud.io/v1beta1
kind: Project
metadata:
  name: ${value("metadata.name", "dev")}<% annotations = value("metadata.annotations", {}); labels = value("metadata.labels", {}) %>
  % if annotations != {}:
  annotations: ${yaml.dump(annotations, width=10000)}
  % endif
  % if labels != {}:
  labels: ${yaml.dump(labels, width=10000)}
  % endif
spec:<% owner = value("spec.owner", {}); description = value("spec.description", ""); purpose = value("spec.purpose", ""); namespace = value("spec.namespace", ""); members = value("spec.members", []) %>
  % if owner != {}:
  owner: ${yaml.dump(owner, width=10000)}
  % else:
  owner:
    apiGroup: rbac.authorization.k8s.io
    kind: User
    name: john.doe@example.com
  % endif
  % if members != []:
  members: ${yaml.dump(members, width=10000)}
  % else:
  members:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: alice.doe@example.com
  % endif
  % if description != "":
  description: ${description}
  % else:
# description: "This is my first project"
  % endif
  % if purpose != "":
  purpose: ${purpose}
  % else:
# purpose: "Experimenting with Gardener"
  % endif
  % if namespace != "":
  namespace: ${namespace}
  % else:
# namespace: "garden-dev" # Optional, will be initialized if unset.
  % endif
