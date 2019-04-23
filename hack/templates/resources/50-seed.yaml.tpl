<%
  import os, yaml

  values={}
  if context.get("values", "") != "":
    values=yaml.load(open(context.get("values", "")), Loader=yaml.Loader)

  if context.get("cloud", "") == "":
    raise Exception("missing --var cloud={aws,azure,gcp,alicloud,openstack,packet} flag")

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

  region=""
  if cloud == "aws":
    region="eu-west-1"
  elif cloud == "azure" or cloud == "az":
    region="westeurope"
  elif cloud == "gcp":
    region="europe-west1"
  elif cloud == "alicloud":
    region="cn-beijing"
    annotations["persistentvolume.garden.sapcloud.io/minimumSize"] = "20Gi"
  elif cloud == "openstack" or cloud == "os":
    region="europe-1"
  elif cloud == "packet":
    region="EWR1"
%># Seed cluster registration manifest into which the control planes of Shoot clusters will be deployed.
---
apiVersion: garden.sapcloud.io/v1beta1
kind: Seed
metadata:
  name: ${value("metadata.name", cloud)}
  % if annotations != {}:
  annotations: ${yaml.dump(annotations, width=1000, default_flow_style=None)}
  % endif
  % if labels != {}:
  labels: ${yaml.dump(labels, width=10000, default_flow_style=None)}
  % endif
spec:
  cloud:
    profile: ${value("spec.cloud.profile", cloud)}
    region: ${value("spec.cloud.region", region)}
  secretRef:
    name: ${value("spec.secretRef.name", "seed-" + cloud)}
    namespace: ${value("spec.secretRef.namespace", "garden")}
  ingressDomain: ${value("spec.ingressDomain", "dev." + cloud + ".seed.example.com")}
  networks: # Seed and Shoot networks must be disjunct
    nodes: ${value("spec.networks.nodes", "10.240.0.0/16")}
    pods: ${value("spec.networks.pods", "10.241.128.0/17")}
    services: ${value("spec.networks.services", "10.241.0.0/17")}
