---
title: 18 Shoot CA Rotation
gep-number: 18
creation-date: 2022-02-01
status: implementable
authors:
- "@beckermax"
- "@rfranzke"
- "@timebertt"
reviewers:
- "@beckermax"
- "@rfranzke"
- "@timebertt"
---

# GEP-18: Automated Shoot CA Rotation

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
- [Alternatives](#alternatives)
- [Open Questions](#open-questions)

## Summary

This proposal outlines an on-demand, multi-step approach to rotate all certificate authorities (CA) used in a Shoot cluster. This process includes creating new CAs, invalidating the old ones and recreating all certificates signed by the CAs.

We propose to bundle the rotation of *all* CAs in the Shoot together as one triggerable action. This includes the recreation and invalidation of the following CAs and all certificates signed by them:

- Cluster CA (currently used for signing `kube-apiserver` serving certificates and client certificates)
- `kubelet` CA (used for signing client certificates for talking to `kubelet` API, e.g. `kube-apiserver-kubelet`)
- `etcd` CA (used for signing `etcd` serving certificates and client certificates)
- front-proxy CA (used for signing client certificates that `kube-aggregator` (part of `kube-apiserver`) uses to talk to extension API servers, filled into `extension-apiserver-authentication` ConfigMap and read by extension API servers to verify incoming `kube-aggregator` requests)
- `metrics-server` CA (used for signing serving certificates, filled into APIService `caBundle` field and read by `kube-aggregator` to verify the presented serving certificate)
- `ReversedVPN` CA (used for signing `vpn-seed-server` serving certificate and `vpn-shoot` client certificate)

Out of scope for now:
- `kubelet` serving CA is self-generated (valid for `1y`) and self-signed by `kubelet` on startup.
  - `kube-apiserver` does not seem to verify the presented serving certificate.
  - `kubelet` can be configured to request serving certificate via CSR that can be verified by `kube-apiserver`, though, we consider this as a separate improvement outside of this GEP.
- Legacy VPN solution uses the cluster CA for both serving and client certificates. As the solution is soon to be dropped in favor of the new `ReversedVPN` solution, we don't intend to introduce a dedicated CA for this component. If `ReversedVPN` is disabled and the CA rotation is triggered, we make sure to propagate the cluster CA to the relevant places in the legacy VPN solution.

Naturally, not all certificates used for communication with the `kube-apiserver` are under control of Gardener. An example for a Gardener-controlled certificate is the kubelet client certificate used to communicate with the api server. An example for credentials not controlled by Gardener are kubeconfigs or client certificates requested via `CertificateSigningRequest`s by the shoot owner.

We propose to use a two step approach to rotate CAs. The start of each phase is triggered by the shoot owner.
In summary, the **first phase** is used to create new CAs (for example, the new api server and client CA). Then we make sure that all servers and clients under Gardener's control trust *both* old and new CA. Next, we renew all client certificates that are under Gardener's control so they are now signed by the new CAs. This includes a node rollout in order to propagate the certificates to kubelets and restart all pods. Afterwards, the user needs to change their client credentials to trust both old and new cluster CA.
In the **second phase**, we remove all trust to the old CA for servers and clients under Gardener's control. This does not include a node rollout but all still running pods using `ServiceAccount`s will continue to trust the old CA until they restart. Also, the user needs to retrieve the new CA bundle to no longer trust the old CA.

A detailed overview of all steps required for each phase is given in the [proposal](#proposal) section of this GEP.

*Introducing a new client CA*

Currently, client certificates and the kube-apiserver certificate are signed by the same CA. We propose to create a separate client CA when triggering the rotation. The client CA is used to sign certificates of clients talking to the API Server.

## Motivation

There are a few reasons for rotating shoot cluster CAs:
- If we have to invalidate client certificates for the kube-apiserver or any other component we are forced to rotate the CA. The only way to invalidate them is to stop trusting all client certificates that are signed by the respective CA, as Kubernetes does not support revoking certificates.
- If the CA itself got leaked.
- If the CA is about to expire.
- If a company policy requires to rotate a CA after a certain point in time.

In each of those cases we currently need to basically manually recreate and replace all CAs and certificates. The process of rotating by hand is cumbersome and could lead to errors due to the many steps needing to be performed in the right order. By automating the process we want to create a way to securely and easily rotate shoot CAs.

### Goals

- Offer an automated and safe solution to rotate all CAs in a shoot cluster.
- Offer a process that is easily understandable for developers and users.
- Rotate the different CAs in the shoot with a similar process to reduce complexity.
- Add visibility for Shoot owners when the last CA rotation happened.

### Non-Goals

- Offer an automated solution for rotating other static credentials (like static token).
  - Later on, a similar two-phase approach could be implemented for the kubeconfig rotation. However, this is out of scope for this enhancement.
- Creating a process that runs fully automated without shoot owner interaction. As the shoot owner controls some secrets, that would probably not even be possible.
- Forcing the shoot owner to rotate after a certain time period. Our goal, rather, is to issue long-running certificates and let the user decide depending on their requirements to rotate as needed.
- Configurable default CA lifetime

## Proposal

We will add a new feature gate `CARotation` for `gardener-apiserver` and `gardenlet`, which allows to enable or disable the possibility to trigger the rotation.

### Triggering the CA Rotation

- Triggered via `gardener.cloud/operation` annotation in symmetry with other operations like reconciliation, kubeconfig rotation, etc.
  - Annotation increases the generation
  - Value for triggering first phase: `start-ca-rotation`
  - Value for triggering the second phase: `complete-ca-rotation`
  - `gardener-apiserver` performs the needful validation: a user can't trigger another rotation if one is already in progress, a user can't trigger `complete-ca-rotation` if first phase has not been compeleted, etc.
- The annotation triggers a usual shoot reconciliation (just like a kubeconfig or SSH key rotation).
- The `gardenlet` begins the CA rotation sequence by setting the new status section `.status.credentials.caRotation` (probably in `updateShootStatusOperationStart`) and removes the annotation afterwards.
  - Shoot reconciliation needs to be idemptotent to CA rotation phase, i.e., if a usual reconciliation or maintenance operation is triggered in between, no new CAs are generated or similar things that would interfere with the CA rotation sequence.

### Changing the Shoot Status

A new section in the Shoot status is added when the first rotation is triggered:

```yaml
status:
  credentials:
    rotation:
      certificateAuthorities:
        phase: Prepare # Prepare|Finalize|Completed
        lastCompletion: 2022-02-07T14:23:44Z
    # kubeconfig:
    #   phase:
    #   lastCompletion:
```

Later on, this section could be augmented with other information, like the names of the credentials secrets (e.g. [gardener/gardener#1749](https://github.com/gardener/gardener/issues/1749))

```yaml
status:
  credentials:
    resources:
    - type: kubeconfig
      kind: Secret
      name: shoot-foo.kubeconfig
```

### Rotation Sequence for Cluster and Client CA

The proposal section includes a detailed description of all steps involved for rotating from a given `CA0` to the target `CA1`.

`t0`: Today's situation:

- `kube-apiserver` uses SERVER CERT signed by `CA0` and trusts CLIENT CERTS signed by `CA0`
- `kube-controller-manager` issues new CLIENT CERTS signed by `CA0`
- kubeconfig trusts only `CA0`
- `ServiceAccount` secrets trust only `CA0`
- kubelet uses CLIENT CERT signed by `CA0`

`t1`: Shoot owner triggers first step of CA rotation process (--> phase one is started):

- Generate `CA1`
- Generate `CLIENT_CA1`
- Update `kube-apiserver`, `kube-scheduler`, etc., to trust CLIENT CERTS signed by both `CA0` and `CLIENT_CA1` (`--client-ca-file` flag)
- Update `kube-controller-manager` to issue new CLIENT CERTS now with `CLIENT_CA1`
- Update kubeconfig so that its CA bundle contains both `CA0` and`CA1` (if kubeconfig still contains a legacy CLIENT CERT then rotate the kubeconfig)
- Update `generic-token-kubeconfig` so that its CA bundle contains both `CA0` and`CA1`
- Update `kube-controller-manager` to populate both `CA0` and `CA1` in `ServiceAccount` secrets
- Restart control plane components so that their CA bundle contains both `CA0` and `CA1`
- Renew CLIENT CERTS (sign them with `CLIENT_CA1`) for the following control plane components: Prometheus, DWD, legacy VPN), if not dropped already in the context of [gardener/gardener#4661](https://github.com/gardener/gardener/issues/4661)
- Trigger node rollout
  - This issues new CLIENT CERTS for all kubelets signed by `CLIENT_CA1`
  - This restarts all `Pod`s and propagates `CA0` and `CA1` into their mounted `ServiceAccount` secrets (note CAs can not be reloaded by go client, therefore we need a restart of pods)
- _Ask the user to exchange all their client credentials (kubeconfig, CLIENT CERTS issued by `CertificateSigningRequest`s) to trust both CA0 and CA1_

`t2`: Shoot owner triggers second step of CA rotation process (--> phase two is started):

Prerequisite: All Gardener-controlled actions listed in `t1` were executed successfully (for example node rollout). The shoot owner has guaranteed that they exchanged their client credentials and triggered step 2 via an annotation.

- Renew SERVER CERTS (sign them with `CA1`) for `kube-apiserver`, `kube-controller-manager`, `cloud-controller-manager`, etc.
- Update `kube-apiserver`, `kube-scheduler`, etc., to trust only CLIENT CERTS signed by `CLIENT_CA1`
- Update kubeconfig so that its CA bundle contains only `CA1`
- Update `generic-token-kubeconfig` so that its CA bundle contains only `CA1`
- Update `kube-controller-manager` to only contain CA1. `ServiceAccount` secrets created after this point will get secrets that include only `CA1`
- Restart control plane components so that their CA bundle contains only `CA1`
- Restart kubelets so that the CA bundle in their kubeconfigs contain only `CA1`
- Delete `CA0`
- _Ask the user to optionally restart their `Pod`s since they still contain `CA0` in memory in order to eliminate trust to the old cluster CA._
- _Ask the user to exchange all their client credentials (download kubeconfig containing only `CA1`; when using CLIENT CERTS trust only `CA1`)_

### Rotation Sequence of Other CAs

Apart from the kube-apiserver CA (and the client CA), we also use 5 other CAs as mentioned above in the Gardener codebase. We propose to rotate those CAs together with the kube-apiserver CA following the same trigger.

:information_source: Note for the front-proxy CA: users need to make sure, extension API servers have reloaded the `extension-apiserver-authentication` ConfigMap, before triggering the second phase.

You can find Gardener managed CAs listed in [wanted_secrets.go](https://github.com/gardener/gardener/blob/04d2b3f459d198e8db0ab57180ca2fea18e84da9/pkg/operation/botanist/wanted_secrets.go#L48).

Regarding the rotation steps, we want to follow a similar approach to the one we defined for the kube-apiserver CA. Exemplary, we are going to show the timeline for ETCD_CA but the logic should be similiar for all the above listed CAs.

- `t0`:
  - etcd trusts client certificates signed by  `ETCD_CA0` and uses a server certificate signed by `ETCD_CA0`
  - `kube-apiserver` and `backup-restore` use a client certificate signed by `ETCD_CA0` and trust `ETCD_CA0`
- `t1`:
  - Generate `ETCD_CA1`
  - Update `etcd` to trust CLIENT CERTS signed by both `ETCD_CA0` and `ETCD_CA1`
  - Update `kube-apiserver` and `backup-restore`:
    - Adapt CA bundle to trust both `ETCD_CA0` and `ETCD_CA1`
    - Renew CLIENT CERTS (sign them with `ETCD_CA1`)
- `t2`:
  - Update `etcd`:
    - Trust only CLIENT CERTS signed by `ETCD_CA1`
    - Renew SERVER CERT (sign it with `ETCD_CA1`)
  - Update `kube-apiserver` and `backup-restore` so that their CA bundle contains only `ETCD_CA1`

:information_source: This means we are requiring two restarts of etcd in total.

## Alternatives

This section presents a different approach to rotate the CAs, which is to _temporarily create a second set of api-servers utilizing the new CA_. After presenting the approach, advantages and disadvantages of both approaches are listed.

`t0`: Today's situation:

- `kube-apiserver` uses SERVER CERT signed by `CA0` and trusts CLIENT CERTS signed by `CA0`
- `kube-controller-manager` issues new CLIENT CERTS with `CA0`
- kubeconfig contains only `CA0`
- `ServiceAccount` secrets contain only `CA0`
- kubelet uses CLIENT CERT signed by `CA0`

`t1`: User triggers first step of CA rotation process (--> phase one):

- Generate `CA1`
- Generate `CLIENT_CA1`
- Create new `DNSRecord`, `Service`, Istio configuration, etc., for second `kube-apiserver` deployment
- Deploy second `kube-apiserver` deployment trusting only CLIENT CERTS signed by `CLIENT_CA1` and using SERVER CERT signed by `CA1`
- Update `kube-scheduler`, etc., to trust only CLIENT CERTS signed by `CLIENT_CA1` (`--client-ca-file` flag)
- Update `kube-controller-manager` to issue new CLIENT CERTS with `CLIENT_CA1`
- Update kubeconfig so that it points to the new `DNSRecord` and its CA bundle contains only `CA1` (if kubeconfig still contains a legacy CLIENT CERT then rotate the kubeconfig)
- Update `ServiceAccount` secrets so that their CA bundle contains both `CA0` and `CA1`
- Restart control plane components so that they point to the second `kube-apiserver` `Service` and so that their CA bundle contains only `CA1`
- Renew CLIENT CERTS (sign them with `CLIENT_CA1`) for control plane components (Prometheus, DWD, legacy VPN) and point them to the second `kube-apiserver` `Service`
- Adapt `apiserver-proxy-pod-mutator` to point `KUBERNETES_SERVICE_HOST` env variable to second `kube-apiserver`
- Trigger node rollout
  - This issues new CLIENT CERTS for all kubelets signed by `CLIENT_CA1` and points them to the second `DNSRecord`
  - This restarts all `Pod`s and propagates `CA0` and `CA1` into their mounted `ServiceAccount` secrets
- _Ask the user to exchange all their client credentials (kubeconfig, CLIENT CERTS issued by `CertificateSigningRequest`s)_

`t2`: User triggers second step of CA rotation process (--> phase two):

- Update `ServiceAccount` secrets so that their CA bundle contains only `CA1`
- Update `apiserver-proxy` to talk to second `kube-apiserver`
- Drop first `DNSRecord`, `Service`, Istio configuration and first `kube-apiserver` deployment
- Drop `CA0`
- _Ask the user to optionally restart their `Pod`s since they still contain `CA0` in memory._


#### Advantages/Disadvantages Approach Two API Servers

- (+) User needs to adapt client credentials only once
- (/) Unstable API server domain
- (-) Probably more implementation effort
- (-) More complex
- (-) CA rotation process does not work similar for all CAs in our system

#### Advantages/Disadvantages of Currently Preferred Approach (See Proposal)

- (+) Implementation effort seems "straight-forward"
- (+) CA rotation process works similar for all CAs in our system
- (/) Stable API server domain
- (-) User needs to adapt client credentials twice
