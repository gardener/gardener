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
apiVersion: settings.gardener.cloud/v1alpha1
kind: OpenIDConnectPreset
metadata:
  name:  ${value("metadata.name", "example-preset")}<% annotations = value("metadata.annotations", {}); labels = value("metadata.labels", {}) %>
  namespace: ${value("metadata.namespace", "garden-dev")}
  % if annotations != {}:
  annotations: ${yaml.dump(annotations, width=1000, default_flow_style=None)}
  % endif
  % if labels != {}:
  labels: ${yaml.dump(labels, width=10000, default_flow_style=None)}
  % endif
shootSelector: # use {} to select all Shoots in that namespace
  matchExpressions:
  - {key: oidc, operator: In, values: [enabled]}
server:
  clientID: client-id
  issuerURL: https://identity.example.com
  # caBundle: |
  #   -----BEGIN CERTIFICATE-----
  #   Li4u
  #   -----END CERTIFICATE-----
  # groupsClaim: groups-claim
  # groupsPrefix: groups-prefix
  # usernameClaim: username-claim
  # usernamePrefix: username-prefix
  # signingAlgs:
  # - RS256
  # only usable with Kubernetes >= 1.11
  # requiredClaims:
  #   key: value
client:
  secret: oidc-client-secret
  extraConfig:
    extra-scopes: email,offline_access,profile
    foo: bar
weight: 90 # value from 1 to 100
