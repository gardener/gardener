---
title: OpenIDConnect Presets
---

# ClusterOpenIDConnectPreset and OpenIDConnectPreset

> [!WARNING]
> OpenID Connect is deprecated in favor of [Structured Authentication configuration](../shoot/shoot_access.md#structured-authentication). Setting OpenID Connect configurations is forbidden for clusters with Kubernetes version `>= 1.32`.

This page provides an overview of ClusterOpenIDConnectPresets and OpenIDConnectPresets, which are objects for injecting [OpenIDConnect Configuration](https://openid.net/connect/) into `Shoot` at creation time. The injected information contains configuration for the Kube API Server and optionally configuration for kubeconfig generation using said configuration.

## OpenIDConnectPreset

An OpenIDConnectPreset is an API resource for injecting additional runtime OIDC requirements into a Shoot at creation time. You use label selectors to specify the `Shoot` to which a given OpenIDConnectPreset applies.

Using a OpenIDConnectPresets allows project owners to not have to explicitly provide the same OIDC configuration for every `Shoot` in their `Project`.

For more information about the background, see the [issue](https://github.com/gardener/gardener/issues/1161) for OpenIDConnectPreset.

### How OpenIDConnectPreset Works

Gardener provides an admission controller (OpenIDConnectPreset) which, when enabled, applies OpenIDConnectPresets to incoming `Shoot` creation requests. When a `Shoot` creation request occurs, the system does the following:

- Retrieve all OpenIDConnectPreset available for use in the `Shoot` namespace.
- Check if the shoot label selectors of any OpenIDConnectPreset matches the labels on the Shoot being created.
- If multiple presets are matched then only one is chosen and results are sorted based on:

  1. `.spec.weight` value.
  1. lexicographically ordering their names (e.g., `002preset` > `001preset`)

- If the `Shoot` already has a `.spec.kubernetes.kubeAPIServer.oidcConfig`, then no mutation occurs.

### Simple OpenIDConnectPreset Example

This is a simple example to show how a `Shoot` is modified by the OpenIDConnectPreset:

```yaml
apiVersion: settings.gardener.cloud/v1alpha1
kind: OpenIDConnectPreset
metadata:
  name:  test-1
  namespace: default
spec:
  shootSelector:
    matchLabels:
      oidc: enabled
  server:
    clientID: test-1
    issuerURL: https://foo.bar
    # caBundle: |
    #   -----BEGIN CERTIFICATE-----
    #   Li4u
    #   -----END CERTIFICATE-----
    groupsClaim: groups-claim
    groupsPrefix: groups-prefix
    usernameClaim: username-claim
    usernamePrefix: username-prefix
    signingAlgs:
    - RS256
    requiredClaims:
      key: value
  weight: 90
```

Create the OpenIDConnectPreset:

```console
kubectl apply -f preset.yaml
```

Examine the created OpenIDConnectPreset:

```console
kubectl get openidconnectpresets
NAME     ISSUER            SHOOT-SELECTOR   AGE
test-1   https://foo.bar   oidc=enabled     1s
```

Simple `Shoot` example:

This is a sample of a `Shoot` with some fields omitted:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: preset
  namespace: default
  labels:
    oidc: enabled
spec:
  kubernetes:
    version: 1.20.2
```

Create the Shoot:

```console
kubectl apply -f shoot.yaml
```

Examine the created Shoot:

```console
kubectl get shoot preset -o yaml
```

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: preset
  namespace: default
  labels:
    oidc: enabled
spec:
  kubernetes:
    kubeAPIServer:
      oidcConfig:
        clientID: test-1
        groupsClaim: groups-claim
        groupsPrefix: groups-prefix
        issuerURL: https://foo.bar
        requiredClaims:
          key: value
        signingAlgs:
        - RS256
        usernameClaim: username-claim
        usernamePrefix: username-prefix
    version: 1.20.2
```

### Disable OpenIDConnectPreset

The OpenIDConnectPreset admission control is enabled by default. To disable it, use the `--disable-admission-plugins` flag on the gardener-apiserver.

For example:

```text
--disable-admission-plugins=OpenIDConnectPreset
```

## ClusterOpenIDConnectPreset

A ClusterOpenIDConnectPreset is an API resource for injecting additional runtime OIDC requirements into a Shoot at creation time. In contrast to OpenIDConnect, it's a cluster-scoped resource. You use label selectors to specify the `Project` and `Shoot` to which a given OpenIDCConnectPreset applies.

Using a OpenIDConnectPresets allows cluster owners to not have to explicitly provide the same OIDC configuration for every `Shoot` in specific `Project`.

For more information about the background, see the [issue](https://github.com/gardener/gardener/issues/1161) for ClusterOpenIDConnectPreset.

### How ClusterOpenIDConnectPreset Works

Gardener provides an admission controller (ClusterOpenIDConnectPreset) which, when enabled, applies ClusterOpenIDConnectPresets to incoming `Shoot` creation requests. When a `Shoot` creation request occurs, the system does the following:

- Retrieve all ClusterOpenIDConnectPresets available.
- Check if the project label selector of any ClusterOpenIDConnectPreset matches the labels of the `Project` in which the `Shoot` is being  created.
- Check if the shoot label selectors of any ClusterOpenIDConnectPreset matches the labels on the `Shoot` being created.
- If multiple presets are matched then only one is chosen and results are sorted based on:

  1. `.spec.weight` value.
  1. lexicographically ordering their names ( e.g. `002preset` > `001preset` )

- If the `Shoot` already has a `.spec.kubernetes.kubeAPIServer.oidcConfig` then no mutation occurs.

> **Note:** Due to the previous requirement, if a `Shoot` is matched by both `OpenIDConnectPreset` and `ClusterOpenIDConnectPreset`, then `OpenIDConnectPreset` takes precedence over `ClusterOpenIDConnectPreset`.

### Simple ClusterOpenIDConnectPreset Example

This is a simple example to show how a `Shoot` is modified by the ClusterOpenIDConnectPreset:

```yaml
apiVersion: settings.gardener.cloud/v1alpha1
kind: ClusterOpenIDConnectPreset
metadata:
  name:  test
spec:
  shootSelector:
    matchLabels:
      oidc: enabled
  projectSelector: {} # selects all projects.
  server:
    clientID: cluster-preset
    issuerURL: https://foo.bar
    # caBundle: |
    #   -----BEGIN CERTIFICATE-----
    #   Li4u
    #   -----END CERTIFICATE-----
    groupsClaim: groups-claim
    groupsPrefix: groups-prefix
    usernameClaim: username-claim
    usernamePrefix: username-prefix
    signingAlgs:
    - RS256
    requiredClaims:
      key: value
  weight: 90
```

Create the ClusterOpenIDConnectPreset:

```console
kubectl apply -f preset.yaml
```

Examine the created ClusterOpenIDConnectPreset:

```bash
kubectl get clusteropenidconnectpresets
NAME     ISSUER            PROJECT-SELECTOR   SHOOT-SELECTOR   AGE
test     https://foo.bar   <none>             oidc=enabled     1s
```

This is a sample of a `Shoot`, with some fields omitted:

```yaml
kind: Shoot
apiVersion: core.gardener.cloud/v1beta1
metadata:
  name: preset
  namespace: default
  labels:
    oidc: enabled
spec:
  kubernetes:
    version: 1.20.2
```

Create the Shoot:

```console
kubectl apply -f shoot.yaml
```

Examine the created Shoot:

```console
kubectl get shoot preset -o yaml
```

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: preset
  namespace: default
  labels:
    oidc: enabled
spec:
  kubernetes:
    kubeAPIServer:
      oidcConfig:
        clientID: cluster-preset
        groupsClaim: groups-claim
        groupsPrefix: groups-prefix
        issuerURL: https://foo.bar
        requiredClaims:
          key: value
        signingAlgs:
        - RS256
        usernameClaim: username-claim
        usernamePrefix: username-prefix
    version: 1.20.2
```

### Disable ClusterOpenIDConnectPreset

The ClusterOpenIDConnectPreset admission control is enabled by default. To disable it, use the `--disable-admission-plugins` flag on the gardener-apiserver.

For example:

```text
--disable-admission-plugins=ClusterOpenIDConnectPreset
```
