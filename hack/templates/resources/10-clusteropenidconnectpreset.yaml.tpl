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
%># ClusterOpenIDConnectPreset is a OpenID Connect configuration that is applied to a Shoot objects cluster-wide.
---
apiVersion: settings.gardener.cloud/v1alpha1
kind: ClusterOpenIDConnectPreset
metadata:
  name:  ${value("metadata.name", "example-preset")}<% annotations = value("metadata.annotations", {}); labels = value("metadata.labels", {}) %>
  % if annotations != {}:
  annotations: ${yaml.dump(annotations, width=1000, default_flow_style=None)}
  % endif
  % if labels != {}:
  labels: ${yaml.dump(labels, width=10000, default_flow_style=None)}
  % endif
spec:
  shootSelector: # use {} to select all Shoots in a matched namespace
    matchExpressions:
    - {key: oidc, operator: In, values: [enabled]}
  projectSelector: # use {} to select all Projects
    matchExpressions:
    - {key: global-oidc, operator: In, values: [enabled]}
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
