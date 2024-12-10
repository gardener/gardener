---
title: Shoot OIDC Issuer
gep-number: 24
creation-date: 2024-01-09
status: implementable
authors:
- "@dimityrmirchev"
reviewers:
- "@vpnachev"
- "@timuthy"
- "@rfranzke"
---

# GEP-24: Shoot OIDC Issuer

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
- [Proposal](#proposal)
    - [Static validation](#static-validation)
    - [Gardenlet](#gardenlet)
    - [Metadata server](#metadata-server)
- [Clarifications](#clarifications)

## Summary

Kubernetes clusters can act as an OIDC compatible provider in a sense that they serve OIDC discovery documents. These documents if publicly accessible can be used by other systems to establish trust to such clusters by fetching the public key part of the key that is used to sign service account tokens. The public keys can be then used by external systems to verify that a service account token is issued by a particular cluster and then grant access to the bearer of that token.

This GEP proposes a mechanism that will allow users to expose such information through Gardener managed domain.

## Motivation

By default Gardener creates clusters which expose such documents directly through the `kube-apiserver`. This has several drawbacks:
- The `kube-apiserver` is protected and unless `--anonymous-auth` is enabled then the endpoint requires authn.
- The certificate used to serve the documents is not signed by a trusted CA.

Gardener provides means for users to tweak configuration and overcome these drawbacks by using the following [shoot configurations](https://github.com/gardener/gardener/blob/580324da9af4ec47955d9e216569d09053c5d008/example/90-shoot.yaml#L201-L204) which directly interact with the [--service-account-issuer](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/) flag of the `kube-apiserver`. Once the issuer of a shoot is changed the user can take over that domain and expose the OIDC metadata documents to the public. This poses another challenge since it requires additional involvement. Some users use istio/nginx igress to do so, others make use of projects like [service-account-issuer-discovery](https://github.com/gardener/service-account-issuer-discovery), or combination of both.

A simpler approach will be for Gardener to be able to take ownership of a shoot's issuer in a central place which can serve all shoots OIDC discovery documents. This will facilitate easier integrations and even allow clusters residing in fenced environments, i.e., those protected by a firewall, to share their OIDC documents. This approach is in sync with the concept of a managed service and is what other Kubernetes offerings as GKE, AKS and EKS already provide.

### Goals
 - Stay backwards compatible with the current configuration options. Existing integrations should continue to work.
 - Provide a secure managed solution for serving the OIDC discovery documents of a shoot cluster.
 - Improve integration capabilities for clusters that reside in a fenced environment.
### Non-Goals
 - Establish trust relations between clusters. This GEP is about simplifying the prerequisites for doing that.

## Proposal

This proposal consists of three parts that together will achieve the mentioned goals.

### Static validation

Static validation rule will be introduced enforcing that `shoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig.Issuer` is not set when the shoot is annotated with `authentication.gardener.cloud/issuer: managed`. This annotation will be incompatible with workerless shoots, as they do not have nodes and they do not run any workloads. The rule will also assure that once present the annotation cannot be removed from a shoot.

```go
if oldShoot.Annotations["authentication.gardener.cloud/issuer"] == "managed" {
    // ensure that the new shoot also has this annotation
    // ensure that newShoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig.Issuer is not set
    // ensure that the shoot is not configured as workerless
    newShoot.Annotations["authentication.gardener.cloud/issuer"] = "managed"
} else if newShoot.Annotations["authentication.gardener.cloud/issuer"] == "managed" {
    // ensure that newShoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig.Issuer is not set
    // ensure that the shoot is not configured as workerless
}
```

### Gardenlet

Once annotated shoots will be easily identifiable by the `gardenlet`. If a shoot requires a managed issuer then the `gardenlet` will configure the `--service-account-issuer` and `--service-account-jwks-uri` flags for the shoot's [kube-apiserver](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/) deployment during reconciliation and set their values to `https://<central.domain.name>/projects/<project-name>/shoots/<shoot-uid>/issuer` and `https://<central.domain.name>/projects/<project-name>/shoots/<shoot-uid>/issuer/jwks` respectively (the central domain name should be configurable via the gardenlet's configuration). After a successful rollout of the `kube-apiserver` deployment, the `gardenlet` will fetch the OIDC metadata discovery documents from a shoot's `kube-apiserver` and sync them back to the Garden cluster in the form of a configmap following the naming convention `projectname--shootuid` in a special purposed namespace called `gardener-system-shoot-issuer`. This namespace will be created by the `gardener-apiserver` during the Garden cluster bootstrap phase. The [Seed Authorizer](https://github.com/gardener/gardener/blob/master/docs/deployment/gardenlet_api_access.md#scoped-api-access-for-gardenlets-and-extensions) will be enhanced so that Gardenlet is only permitted to perform actions on related configmaps. An additional advertised address will be added to the shoot status so that the issuer of the cluster is easily discoverable.

```yaml
status:
  advertisedAddresses:
    ...
    - name: service-account-issuer
      url: https://<central.domain.name>/projects/<project-name>/shoots/<shoot-uid>/issuer
```

During the deletion flow of a shoot the previously synced configmap will be deleted from the Garden cluster.

The contents of the described configmap will contain both the `/.well-known/openid-configuration` and the `/jwks` metadata. It is important to note that the issuer URL uses the shoot UID in its path. This is done so that a recreated shoot has a different issuer compared to its predecessor with the same name. See the following example.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: myproj--f924a208-1034-4aeb-84d9-b0184894a0cf
  namespace: gardener-system-shoot-issuer
  labels:
    project.gardener.cloud/name: myproj
    shoot.gardener.cloud/name: myshoot
data:
  openid-config: |
    {
        "issuer": "https://<central.domain.name>/projects/myproj/shoots/f924a208-1034-4aeb-84d9-b0184894a0cf/issuer",
        "jwks_uri": "https://<central.domain.name>/projects/myproj/shoots/f924a208-1034-4aeb-84d9-b0184894a0cf/issuer/jwks",
        "response_types_supported": [
            "id_token"
        ],
        "subject_types_supported": [
            "public"
        ],
        "id_token_signing_alg_values_supported": [
            "RS256"
        ]
    }
  jwks: |
    {
        "keys": [
            {
                "use": "sig",
                "kty": "RSA",
                "kid": "<THE_KEY_IDENTIFIER>",
                "alg": "RS256",
                "n": "<THE_PUBLIC_KEY>",
                "e": "AQAB"
            }
        ]
    }
```

### Metadata Server

A new component called `gardener-metadata-server` will be introduced. This component will be maintained in a separate repository in order to decouple its development and release schedule from that of Gardener. It is natural that this component is deployed in the Garden cluster alongside with the the Gardener controlplane components. It will be managed by `gardener-operator`, or can be installed manually when not making use of it for managing the Gardener control plane. The server will have minimal permissions and restricted access to the Garden cluster, i.e. it will only require read access for configmaps in the `gardener-system-shoot-issuer` namespace. The server will be publicly accessible and will serve the metadata information for different shoot clusters, i.e. when a `GET` request hits `https://<central.domain.name>/projects/myproj/shoots/f924a208-1034-4aeb-84d9-b0184894a0cf/issuer/.well-known/openid-configuration` the server should return the contents of `.data.openid-config` from the corresponding configmap. Since this server will be part of authentication flows it needs to be highly available and implemented with security and observability in mind.

## Clarifications

**Question:** Can the discovery configmaps be stored in the project namespace together with the shoot definitions?

**Answer:** The main motivation for having a dedicated namespace that stores the discovery documents configmaps is that it is not safe to store them in the project namespace together with the shoot definitions. If that was the case a malicious actor who has access to a certain configmap can tamper with the contents of it and force the metadata server to publicly serve the tampered public key. This means that such actor can generate a signing key, replace the authentic public key in the configmap, use the private key to sign tokens imitating service account tokens issued by the shoot's `kube-apiserver` and present them to external systems. These systems having already established a trust to the tampered issuer will accept these tokens and give them access as if they were legit, when in fact these tokens were actually signed by a malicious actor.
