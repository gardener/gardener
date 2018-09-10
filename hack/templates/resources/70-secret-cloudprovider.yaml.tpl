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

  entity=""
  if cloud == "aws":
    entity="AWS account"
  elif cloud == "azure" or cloud == "az":
    entity="Azure subscription"
  elif cloud == "gcp":
    entity="GCP project"
  elif cloud == "alicloud":
    entity="Alicloud project"
  elif cloud == "openstack" or cloud == "os":
    entity="OpenStack tenant"
%>---<% if entity != "": print("# Secret containing cloud provider credentials for " + entity + " into which Shoot clusters should be provisioned.") %>
apiVersion: v1
kind: Secret
metadata:
  name: ${value("metadata.name", "core-" + cloud)}
  namespace: ${value("metadata.namespace", "garden-dev")}<% annotations = value("metadata.annotations", {}); labels = value("metadata.labels", {}) %>
  % if annotations != {}:
  annotations: ${yaml.dump(annotations, width=10000)}
  % endif
  % if labels != {}:
  labels: ${yaml.dump(labels, width=10000)}
  % else:
  labels:
    cloudprofile.garden.sapcloud.io/name: ${cloud} # label is only meaningful for Gardener dashboard
  % endif
type: Opaque
data:
  % if cloud == "aws":
  accessKeyID: ${value("data.accessKeyID", "base64(access-key-id)")}
  secretAccessKey: ${value("data.secretAccessKey", "base64(secret-access-key)")}
  % endif
  % if cloud == "azure" or cloud == "az":
  tenantID: ${value("data.tenantID", "base64(uuid-of-tenant)")}
  subscriptionID: ${value("data.subscriptionID", "base64(uuid-of-subscription)")}
  clientID: ${value("data.clientID", "base64(uuid-of-client)")}
  clientSecret: ${value("data.clientSecret", "base64(client-secret)")}
  % endif
  % if cloud == "gcp":
  serviceaccount.json: ${value("data.serviceaccountjson", "base64(serviceaccount-json)")}
  % endif
  % if cloud == "alicloud":
  accessKeyID: ${value("data.accessKeyID", "base64(access-key-id)")}
  accessKeySecret: ${value("data.accessKeySecret", "base64(access-key-secret)")}
  % endif
  % if cloud == "openstack" or cloud == "os":
  domainName: ${value("data.domainName", "base64(domain-name)")}
  tenantName: ${value("data.tenantName", "base64(tenant-name)")}
  username: ${value("data.username", "base64(username)")}
  password: ${value("data.password", "base64(password)")}
  % endif
  % if cloud == "local":
  % endif
