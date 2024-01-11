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
---

# GEP-24: Shoot OIDC Issuer

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
- [Proposal](#proposal)
    - [Admission plugin](#admission-plugin)
    - [Gardenlet](#gardenlet)
    - [Metadata server](#metadata-server)
- [Alternatives](#alternatives)

## Summary

Kubernetes clusters can act as an OIDC compatible provider in a sense that they serve OIDC discovery documents. These documents if publicly accessible can be used by other systems to establish trust to such clusters by fetching the public key part of the key that is used to sign service account tokens. The public keys can be then used by external systems to verify that a service account token is issued by a particular cluster and then grant access to the bearer of that token.

This GEP proposes a mechanism that will allow users to expose such information through Gardener managed domain.

## Motivation

By default Gardener creates clusters which expose such documents directly through the `kube-apiserver`. This has several drawbacks:
- The `kube-apiserver` is protected and unless `--annonymous-auth` is enabled then the endpoint requires authn.
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

Static validation rule will be introduced enforcing that `shoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig.Issuer` is not set when the shoot is annotated with `gardener.cloud/managed-issuer: true`. The rule will also assure that once present the annotation cannot be removed from a shoot.

```go
if oldShoot.Annotations["gardener.cloud/managed-issuer"] == "true" {
    // ensure that the new shoot also has this annotation
    // ensure that newShoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig.Issuer is not set
    newShoot.Annotations["gardener.cloud/managed-issuer"] = "true"
} else if newShoot.Annotations["gardener.cloud/managed-issuer"] == "true" {
    // ensure that newShoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig.Issuer is not set
}
```

### Gardenlet

Once annotated shoots will be easily identifiable by the `gardenlet`. If a shoot requires a managed issuer then the `gardenlet` will configure the `--service-account-issuer` and `--service-account-jwks-uri` flags for the shoot's [kube-apiserver](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/) deployment during reconciliation and set their values to `https://<central.domain.name>/projects/<project-name>/shoots/<shoot-uid>/issuer` and `https://<central.domain.name>/projects/<project-name>/shoots/<shoot-uid>/issuer/jwks` respectively (the central domain name should be configurable). At the end of a shoot reconciliation flow, the `gardenlet` will fetch the OIDC metadata discovery documents from a shoot's `kube-apiserver` and sync them back to the Garden cluster in the form of a configmap following the naming convention `projectname--shootuid` in a special purposed namespace called `gardener-shoot-issuer`(name should be configurable). An additional advertised address will be added to the shoot status so that the issuer of the cluster is easily discoverable.

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
  namespace: gardener-shoot-issuer
  labels:
    gardener.cloud/project: myproj
    gardener.cloud/shootName: myshoot
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
                "kid": "<THE_KEY_INDENTIFIER>",
                "alg": "RS256",
                "n": "<THE_PUBLIC_KEY>",
                "e": "AQAB"
            }
        ]
    }
```

### Metadata Server

A new component called `gardener-metadata-server` will be introduced. It is natural that this component is deployed in the Garden cluster alongside with the rest of the Gardener controlplane components. It can also be managed by the `gardener-operator`. The server will have minimal permissions and restricted access to the Garden cluster, i.e. it will only require read access for configmaps in the `gardener-shoot-issuer` namespace. The server will be publicly accessible and will serve the metadata information for different shoot clusters, i.e. when a `GET` request hits `https://<central.domain.name>/projects/myproj/shoots/f924a208-1034-4aeb-84d9-b0184894a0cf/issuer/.well-known/openid-configuration` the server should return the contents of `.data.openid-config` from the corresponding configmap. Since this server will be part of authentication flows it needs to be highly available and implemented with security and observability in mind.

## Alternatives
There were no previous discussions of alternatives.
