---
title: Shoot CA Rotation
gep-number: 18
creation-date: 2022-02-01
status: implementable
authors:
- "@beckermax"
- "@rfranzke"
reviewers:
- 
---

# Automated Shoot CA rotation

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
- [Proposal](#proposal)
- [Alternatives](#alternatives)
- [Open Questions](#open-questions)

## Summary

This proposal outlines an on-demand multi-step approach to rotate all certificate authorities (CA) used in a Shoot cluster. This process includes creating new CAs, invalidating the old ones and recreating all certs signed by the CAs. 

We propose to bundle the rotation of *all* CAs in the Shoot together as one trigerrable action. This includes the recreation and invalidation of the following CAs and all certs signed by them:

- shoot kube-api-server CA
- kubelet CA
- Etcd CA
- Front proxy CA
- Metrics server CA
- VPN CA

Naturally not all certificates communicating with the `kube-api-server` are under control of gardener. An example for a gardener controlled cert is the kubelet client certificate used to communicate with the api server. An example for credentials not controlled by gardener are kubeconfigs or client certs requested via `CertificateSigningRequest`s by the shoot owner.

We propose to use a two step approach to rotate CAs. The start of each phase is triggered by the shoot owner. In summary the **first phase** is used to create new CAs (for example the new api server and client CA). Then we make sure that all server and clients trust *both* old and new CA. Next we renew all client certs and server certs so they are now signed by the new CAs. This includes a node rollout in order to propagate the certs to kubelets and restart all pods. Afterwards the user needs to change their client credentials to trust both old and new apiserver CA.
In the **second phase** we basically remove all trust to the old CA for server and clients. This does not include a node rollout but all pods using SAs will continue to trust the old CA until they restart. Also the user needs to exchange all their client credentials again to no longer trust the old CA.

A detailed overview of all steps required for each phase is given in the [proposal](#proposal) section of this GEP.

*Introducing a new client CA*

Currently client certificates and api server certificate are signed by the same CA. We propose to create a seperate client CA when triggering the rotation. The client CA is used to sign certs of clients talking to the API Server.

## Motivation

If we face the challenge to invalidate a CA be it due to company policy or due to a security incident we need to basically manually recreate and replace all CAs and certificates. The process of rotating by hand is cumbersome and could lead to errors due to the many steps needing to be performed in the right order. By automating the process we want to create a way to securely and easily rotate shoot CAs.

### Goals
- Offer a trigerable and safe solution to rotate all CAs in a shoot cluster.
- Offer a process that is easily understandable for developers and stakeholders
- Rotate the different CAs in the shoot with a similiar process to reduce complexity.

### Non-Goals

- Creating a process that runs fully automated without shoot owner interaction. As the shoot owner controls some secrets that would probably not even be possible.
- Forcing the shoot owner to rotate after a certain time period. Our goal rather is to issue long running certificates and let the user decide depending on their requirements to rotate as needed.

## Proposal

#### kube-api server CA rotation

The propsal section includes a detailed description of all steps involved for rotating from a given `CA0` to the target `CA1`.

`t0`: Today's situation

- `kube-apiserver` uses SERVER CERT signed by `CA0` and trusts CLIENT CERTS signed by `CA0`
- `kube-controller-manager` issues new CLIENT CERTS with `CA0`
- kubeconfig trusts only `CA0`
- `ServiceAccount` secrets trust only `CA0`
- kubelet uses CLIENT CERT signed by `CA0`

`t1`: Shoot owner triggers first step of CA rotation process (--> phase one is started):

- Generate `CA1`
- Generate `CLIENT_CA1`
- Update `kube-apiserver`, `kube-scheduler`, etc. to trust CLIENT CERTS signed by both `CA0` and `CLIENT_CA1` (`--client-ca-file` flag)
- Update `kube-controller-manager` to issue new CLIENT CERTS now with `CLIENT_CA1`
- Update kubeconfig so that its CA bundle contains both `CA0` and`CA1` (if kubeconfig still contains a legacy CLIENT CERT then rotate the kubeconfig)
- Update `kube-controller-manager`to include   both `CA0` and `CA1` in `ServiceAccount` secrets. Then update `Service Account` secrets.
- Restart control plane components so that their CA bundle contains both `CA0` and `CA1` 
- Renew CLIENT CERTS (sign them with `CLIENT_CA1`) for control plane components (Prometheus, DWD, legacy VPN)
- Trigger node rollout
  - This issues new CLIENT CERTS for all kubelets signed by `CLIENT_CA1`
  - This restarts all `Pod`s and propagates `CA0` and `CA1` into their mounted `ServiceAccount` secrets (note CAs can not be reloaded by go client, therefore we need a restart of pods.)
- _Ask user to exchange all their client credentials (kubeconfig, CLIENT CERTS issued by `CertificateSigningRequest`s) to trust both CA0 and CA1_

`t2`: Shoot owner triggers second step of CA rotation process (--> phase two is started):

​	Prerequisite: The shoot owner has guaranteed that they exchanged their client credentials and triggered step 2 via an annotation.

- Renew SERVER CERTS (sign them with `CA1`) for `kube-apiserver`, etc.
- Update `kube-apiserver`, `kube-scheduler`, etc. to trust only CLIENT CERTS signed by `CLIENT_CA1`
- Update kubeconfig so that its CA bundle contains only `CA1`
- Update `kube-controller-manager` to only contain CA1. `ServiceAccount` secrets created after this point will get secrets that include only `CA1`
- Restart control plane components so that their CA bundle contains only `CA1`
- Restart kubelets so that the CA bundle in their kubeconfigs contain only `CA1`
- Delete `CA0`
- _Ask user to optionally restart their `Pod`s since they still contain `CA0` in memory._
- _Ask user to exchange all their client credentials (download kubeconfig containing only `CA1`; when using CLIENT CERTS trust only `CA1`)_

#### Rotation of othere CAs

Apart from the kube-apiserver CA (and the client CA) we also use 5 other CAs in the gardener codebase. We propose to rotate those CAs together with the apisererver CA following the same trigger. The 5 CAs that need rotation are:

- kubelet CA
- etcd CA
- Front proxy CA
- metrics server CA
- VPN CA

You can find gardener managed CAs listed [here](https://github.com/gardener/gardener/blob/04d2b3f459d198e8db0ab57180ca2fea18e84da9/pkg/operation/botanist/wanted_secrets.go#L48).

Regarding the rotation steps we want to follow a similiar approach to the one we defined for the kube-apiserver CA. Exemplarily, we are going to show the timeline for ETCD_CA but the logic should be similiar for all the above listed CAs.

- `t0`
  - etcd trusts client certs signed by  `ETCD_CA0` and uses a server cert signed by `ETCD_CA0`
  - `kube-apiserver` and `backup-restore` use a client cert signed by `ETCD_CA0` and trust `ETCD_CA0`
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

:information_source: This means we are requiring two restarts of etcd

## Alternatives

This section presents a different approach to rotate the CAs which is to _temporarily create  a second set of apiservers utilizing the new CA_ . After presenting the approach advantages and disadvantages of both approaches are listed.

`t0`: Today's situation

- `kube-apiserver` uses SERVER CERT signed by `CA0` and trusts CLIENT CERTS signed by `CA0`
- `kube-controller-manager` issues new CLIENT CERTS with `CA0`
- kubeconfig contains only `CA0`
- `ServiceAccount` secrets contain only `CA0`
- kubelet uses CLIENT CERT signed by `CA0`

`t1`: User triggers first step of CA rotation process (--> phase one):

- Generate `CA1`
- Generate `CLIENT_CA1`
- Create new `DNSRecord`, `Service`, Istio configuration, etc. for second `kube-apiserver` deployment
- Deploy second `kube-apiserver` deployment trusting only CLIENT CERTS signed by `CLIENT_CA1` and using SERVER CERT signed by `CA1`
- Update `kube-scheduler`, etc. to trust only CLIENT CERTS signed by `CLIENT_CA1` (`--client-ca-file` flag)
- Update `kube-controller-manager` to issue new CLIENT CERTS with `CLIENT_CA1`
- Update kubeconfig so that it points to the new `DNSRecord` and its CA bundle contains only `CA1` (if kubeconfig still contains a legacy CLIENT CERT then rotate the kubeconfig)
- Update `ServiceAccount` secrets so that their CA bundle contains both `CA0` and `CA1`
- Restart control plane components so that they point to the second `kube-apiserver` `Service` and so that their CA bundle contains only `CA1`
- Renew CLIENT CERTS (sign them with `CLIENT_CA1`) for control plane components (Prometheus, DWD, legacy VPN) and point them to the second `kube-apiserver` `Service`
- Adapt `apiserver-proxy-pod-mutator` to point `KUBERNETES_SERVICE_HOST` env variable to second `kube-apiserver`
- Trigger node rollout
  - This issues new CLIENT CERTS for all kubelets signed by `CLIENT_CA1` and points them to the second `DNSRecord`
  - This restarts all `Pod`s and propagates `CA0` and `CA1` into their mounted `ServiceAccount` secrets
- _Ask user to exchange all their client credentials (kubeconfig, CLIENT CERTS issued by `CertificateSigningRequest`s)_

`t2`: User triggers second step of CA rotation process (--> phase two):

- Update `ServiceAccount` secrets so that their CA bundle contains only `CA1`
- Update `apiserver-proxy` to talk to second `kube-apiserver`
- Drop first `DNSRecord`, `Service`, Istio configuration and first `kube-apiserver` deployment
- Drop `CA0`
- _Ask user to optionally restart their `Pod`s since they still contain `CA0` in memory._



#### Advantages/Disadvantages approach two api servers

- (+) User needs to adapt client credentials only once
- (/) Unstable API server domain
- (-) Probably more implementation effort
- (-) More complex
- (-) CA rotation process does not work similar for all CAs in our system

#### Advantages/Disadvantages of currently preferred approach (see proposal)

- (+) Implementation effort seems "straight-forward"
- (+) CA rotation process works similar for all CAs in our system
- (/) Stable API server domain
- (-) User needs to adapt client credentials twice
 
## Open Questions

- Should we combine kubeconfig token rotation (not graceful today) with the 2-phase CA rotation process?
- Do we need to mention the "`Pod`s still contain `CA0` in memory" part in the docs (GKE does not)?
