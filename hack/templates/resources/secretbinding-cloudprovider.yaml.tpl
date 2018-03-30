<%
  import os, yaml

  values={}
  if context.get("values", "") != "":
    values=yaml.load(open(context.get("values", "")))

  if context.get("cloud", "") == "":
    raise Exception("missing --var cloud={aws,azure,gcp,openstack,local} flag")

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

  entity=""
  if cloud == "aws":
    entity="AWS account"
  elif cloud == "azure" or cloud == "az":
    entity="Azure subscription"
  elif cloud == "gcp":
    entity="GCP project"
  elif cloud == "openstack" or cloud == "os":
    entity="OpenStack tenant"
%># SecretBindings bind a secret from the same or another namespace together with Quotas from the same or other namespaces.
---
apiVersion: garden.sapcloud.io/v1beta1
kind: SecretBinding
metadata:
  name: ${value("metadata.name", "core-" + cloud)}
  namespace: ${value("metadata.namespace", "garden-dev")}
  labels:
    cloudprofile.garden.sapcloud.io/name: ${cloud} # label is only meaningful for Gardener dashboard
secretRef:
  name: ${value("secretRef.name", "core-" + cloud)}<% secretRefNamespace=value("secretRef.namespace", ""); quotas=value("quotas", []) %>
  % if secretRefNamespace != "":
  namespace: ${secretRefNamespace}
  % else:
# namespace: namespace-other-than-'${value("metadata.namespace", "garden-dev")}' // optional
  % endif
quotas: []
% if len(quotas) == 0:
# - name: quota-1
# # namespace: namespace-other-than-'${value("metadata.namespace", "garden-dev")}' // optional
% endif
