---
title: Workload Identity - Trust Based Authentication
gep-number: 26
creation-date: 2024-02-26
status: implementable
authors:
- "@vpnachev"
reviewers:
- "@dimityrmirchev"
---

# GEP-26: Workload Identity - Trust Based Authentication

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [API Changes](#api-changes)
    - [Shoot API Related Changes](#shoot-api-related-changes)
    - [Seed API Related Changes](#seed-api-related-changes)
  - [Gardener as OIDC Token Issuer](#gardener-as-oidc-token-issuer)
  - [Distribution of Workload Identity Tokens](#distribution-of-workload-identity-tokens)
  - [Use cases](#use-cases)
- [Alternatives](#alternatives)
  - [SPIFFE/SPIRE](#spiffespire)
  - [Kubernetes Service Account Tokens From Garden Cluster](#kubernetes-service-account-tokens-from-garden-cluster)
  - [Kubernetes Service Account Tokens From Seed Cluster](#kubernetes-service-account-tokens-from-seed-cluster)

## Summary

Gardener issues and distributes JSON Web Tokens that can be used for
authentication with external services. Gardener also exposes metadata documents
in an OIDC compatible way, if needed, in the public internet. This allows
Gardener users to establish trust towards Gardener through their service
providers leveraging identity federation and parts of the OIDC protocol. By
employing the JWTs and the trust federation, static credentials are no longer
needed for authentication with the service provider, i.e. they can be replaced
by tokens issued by Gardener which will be recognized by the external entity
because trust was established beforehand. For example, machine controller
manager will no longer require static credentials to create virtual machines in
the cloud provider account of the Gardener user.

Cloud services, like AWS, Azure, Alicloud, GCP and others, support identity
federation with identity providers external to them via trust configuration
using the OIDC protocol. This way, the remotely running workloads can use JWTs
issued from the external identity provider to authenticate with the cloud
service, hence no static credentials like service keys are used. More details
for the cloud providers, can be found at their documentation:

- [AWS: Creating OpenID Connect (OIDC) identity providers](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc.html)
- [Azure: Workload identity federation](https://learn.microsoft.com/en-us/entra/workload-id/workload-identity-federation)
- [Alicloud: Overview of OIDC-based SSO](https://www.alibabacloud.com/help/en/ram/user-guide/overview-of-oidc-based-sso)
- [GCP: Workload identity federation](https://cloud.google.com/iam/docs/workload-identity-federation)

## Motivation

Gardener is using variety of external services to create different resources
needed for the lifecycle of a shoot or seed cluster, resources like virtual
machines, volumes, object storage, load balancers, DNS records, etc. Each
request to create, update or delete such resource needs to be authenticated with
some kind of credentials. Sometimes the resources reside in different cloud
accounts and credentials with mixed ownership are used, for example when
Gardener Operator and Gardener User are different entities, Gardener Users
brings their own credentials for their account and let Gardener use them on
their behalf to create resources. These credentials usually have long lifetime
(and are often non-expiring), they are reused in different scenarios by various
tools, occasionally granted with broader permissions, stored in different
locations. Such handling poses various security risks.

The static long-lived credentials can be replaced with short-lived auto-rotated
JWTs issued and used only by Gardener, never leaving the Gardener environment
except for representing Gardener workloads before an external entity,
eliminating the security burden to manage and store static credentials. Static
credentials can expire or get accidentally invalidated, which will cause
reconciliation flows to fail preventing delivery of updates, fixes, and
improvements. This risk is better to be managed in an automated way by Gardener
itself.

The JSON Web Tokens are ephemeral and not stored anywhere by the issuer, a
feature that Gardener can benefit of as well because it will not have to store
the credentials of Gardener users.

### Goals

- Manage shoot clusters without credentials provided by the user.
- Replace static credentials for gardener system components, e.g. DNS and backup
  controllers.
- Rotate credentials regularly. Rotating the token signing keys will effectively
  invalidate all previously issued tokens.
- Offload Gardener users from the burden to store and manage static credentials
  for their accounts.

### Non-Goals

- Register Gardener as trusted identity in the shoot clusters.
- The tokens to be usable for authentication with the Gardener API.
- Compatibility with gardenctl integration with cloud provider CLIs. JSON Web
  Tokens will not be drop-in replacement of the static credentials, therefore
  gardenctl and other tools will have to adapt to the JWTs as infrastructure
  credentials.
- OIDC compliance - just as Kubernetes, Gardener goal is not to have full OIDC
  compliance, but to implement the bare minimum for OIDC compatible trust
  federation.

## Proposal

In short, Gardener API server will generate JWTs on request by gardenlet.
Gardenlet will ensure that the tokens can be consumed conveniently by the
various components in the seed clusters. For example, it will write them into
secrets. Service provider extensions will be responsible for making adjustments
so that the token is consumable by the service SDK. For shoot clusters, it would
be natural that the `cloudprovider` secret is reused as storage target for the
token.

Gardenlet will take care to refresh the token regularly and on time so that the
target storage always contains a valid token. As an example a token can be
refreshed when it reaches 80% of its lifetime. Of course it will all depend on
the validity duration of such tokens. This is why these parameters will be
configurable.

The OIDC metadata discovery documents will be served in such network segment,
e.g. public internet, so that service providers can be configured to trust
Gardener as an OIDC compatible token issuer.

### API Changes

A new resource `WorkloadIdentity` in `security.gardener.cloud` API Group
will be implemented. It will specify different characteristics of the JWT, like
the value for the `aud` claim.

Similarly to `providerConfig` in other Gardener APIs, `WorkloadIdentity`
resource will feature a `providerConfig` field that will be of byte array type
allowing service provider specific configurations. Usually, the clients for
services supporting identity federation need additional information about the
cloud account and the federated identity in order to successfully use the JWT.
This information is known to the cloud account owners and they will provide it
via this `providerConfig` field, for example when AWS is the external service
the AWS IAM Role ARN will be set in the `providerConfig` field.

The value of the `sub` claim of the OIDC token will be computed by Gardener, it
will have the following format
`gardener.cloud:workloadidentity:<workloadidentity-namespace-name>:<workloadidentity-name>:<workloadidentity-uuid>`.
A validation must ensure that the `WorkloadIdentity` name and namespace name do
not exceed certain limit. This restrictions is required as per the OIDC
[Specification](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
the `sub` claim cannot exceed 255 ASCII chars length. Gardener API server will
write the value of the sub claim in the `status.sub` field to make it explicit,
otherwise Gardener users will have to deduce it themselves which could turn out
to be error prone.

```yaml
apiVersion: security.gardener.cloud/v1alpha1
kind: WorkloadIdentity
metadata:
  name: banana-testing
  namespace: garden-local
  uid: 12b580fe-1f74-4195-852b-e1a74b03496a # generated by the API server.
spec:
  audiences: # Required field.
  - team-foo
  targetSystem: # Required field.
    type: aws # Required field.
    providerConfig: # Optional field of type *runtime.RawExtension, extensions can make it mandatory via admission webhooks.
      apiVersion: aws.provider.extensions.gardener.cloud/v1alpha1
      kind: WorkloadIdentityConfig
      iamRoleARN: arn:aws:iam::112233445566:role/gardener-dev
status:
  sub: gardener.cloud:workloadidentity:garden-local:banana-testing:12b580fe-1f74-4195-852b-e1a74b03496a
```

JWTs will be available when the clients send `create` requests on the
`WorkloadIdentity/token` subresource. As the clients will be providing various
custom information that will be used for the generation of the JWT, yet another
resource `TokenRequest` in the API group `security.gardener.cloud` will be used,
similar to `TokenRequest` from `authentication.k8s.io/v1` API. It is envisioned
this resource to contain just metadata for the context where the JWT is being
used, e.g. shoot or backup entry identifier. Gardener API server must verify the
provided metadata and it can enhance the JWT with additional information derived
from the context, for example with information for the project and the seed of
the shoot cluster. Gardener API can also add global information like a garden
cluster identity. `TokenRequest` will feature optional field `expirationSeconds`
that will allow clients to specify for how long the issued workload identity
token to be valid. This duration will be ensured to be between certain limits of
minimal and maximal validity, in order to avoid frequent token renewals as well
as tokens with too long validity. If the `expirationSeconds` field is not set, a
default duration of 3600 seconds will be applied.

`TokenRequest` resources will never be persisted in the storage layer, the
generated token will be written in the `.status.token` field and returned to the
client as response. The expiration timestamp of the token will be also available
in the status via the `.status.expirationTimestamp` field.

```yaml
apiVersion: security.gardener.cloud/v1alpha1
kind: TokenRequest
spec:
  contextObject: # Optional field, various metadata about context of use of the token
    apiVersion: core.gardener.cloud/v1beta1
    kind: Shoot
    name: foo
    namespace: garden-local
    uid: 54d09554-6a68-4f46-a23a-e3592385d820
  expirationSeconds: 600 # Optional field, gardener will set default value of 3600 seconds for token duration if the field is unset.
status:
  token: eyJhbGciOiJ....OkBBrVWA # The generated OIDC token
  expirationTimestamp: 2024-02-09T16:35:02Z
```

#### Shoot API Related Changes

Currently, shoot clusters set the infrastructure credentials via an intermediate
resource named `SecretBinding` which is referring to the actual Kubernetes
secret that contains the static credentials. If `SecretBinding` is extended to
refer `WorkloadIdentity` as infrastructure credentials, from user experience
point of view `SecretBinding` is not the best name for such resource, because it
is no longer limited to referring only secrets as its name implies. Therefore, a
new resource named `CredentialsBinding` in the API group
`security.gardener.cloud` will be implemented. It will have all features
of `SecretBinding`, but on top of that will be extended to refer to
`WorkloadIdentity` resources via `.credentialsRef` field of type
`ObjectReference`.

In a nutshell, the changes introduced compared to `SecretBinding` are:

- `CredentialsBinding.credentialsRef` field will be mutable. It will refer to a
  `WorkloadIdentity` or `Secret` resource by its name and namespace. If the
  namespace is unset, the namespace of the `CredentialsBinding` will be used.
- `quotas` and `provider` fields have the semantic as their respective
  counterparts in the `SecretBinding` API. `providers` will be made mandatory
  field also via the API specification.
- On update of the `credentialsRef` when `WorkloadIdentity` is the credentials
  provider, extension admission controller should ensure that both the old and
  the new `WorkloadIdentity` are for the same cloud provider account, if such
  validation is possible for the given extension.

```yaml
apiVersion: security.gardener.cloud/v1alpha1
kind: CredentialsBinding
metadata:
  name: my-credentials
  namespace: garden-local
provider:
  type: aws # {aws,azure,gcp,...}
credentialsRef:
  apiVersion: security.gardener.cloud/v1alpha1 # or "v1", when secret is being used
  kind: WorkloadIdentity # or "Secret", when secret is being used
  name: my-provider-account
  # namespace: "...", allow reference across namespaces
quotas: []
# - name: quota-1
# # namespace: garden-quotas
```

Shoot API will be extended with new field `.spec.credentialsBindingName` whose
value will be the name of a `CredentialsBinding` resource from the namespace of
the shoot resource. The shoot field `.spec.secretBindingName` and the
`SecretBinding` API will be deprecated in favour of `CredentialsBinding`, and
eventually removed in a future version of Gardener. A static validation will
ensure exactly one of the fields `.spec.secretBindingName` and
`.spec.credentialsBindingName` is set. `secretBindingName` will be made mutable
to allow already existing shoot clusters to migrate to credentials binding.
Shoots will be able to use workload identity as infrastructure credentials only
via the `CredentialsBinding` resource.

Features associated with the SecretBinding like the
[shoot quotas](../concepts/apiserver.md#shoot-quotas) will be extended to also
cover clusters using WorkloadIdentity for authentication.

While the infrastructure credentials for the shoot cluster are the main driver
behind this GEP, various extensions can benefit of this feature as well. For
this purpose, a new optional field named `workloadIdentity` will be introduced
in `shoot.spec.extensions`, it will refer to workload identity by name assuming
the workloadIdentity resource is in the namespace of the shoot. The seed CRD
`extensions.extensions.gardener.cloud` will be also extended to reflect that the
extension is using workload identity letting extension controllers know when a
JWT or other kind of credentials are used.

Similar approach can be taken to provide alternative to
`shoot.spec.dns.providers.secretName`, e.g. a new field `workloadIdentity` will
extended the `shoot.spec.dns.providers`.

```yaml
spec:
  extensions:
    - type: some-extension
      workloadIdentity:
        name: foo
  dns:
    providers:
    - type: some-dns-provider
      workloadIdentity:
        name: bar
```

#### Seed API Related Changes

Wherever the Seed API is referring to secrets, it will be extended to refer to
workload identities, as of now these are the fields `spec.backup.` and
`spec.dns.provider` and they will have new field `workloadIdentityRef` holding
the name and the namespace of a workload identity resource. The respective
`secretRef` fields will be made optional and validation will ensure only one of
`secretRef` and `workloadIdentityRef` is used at a time.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Seed
metadata:
  name: seed
spec:
  backup:
    workloadIdentityRef:
      name: backup-workloadidentity
      namespace: garden
  dns:
    provider:
      workloadIdentityRef:
        name: ingress-workloadidentity
        namespace: garden
```

### Gardener as OIDC Token Issuer

A new component, positively the metadata server from
[GEP-24](./24-shoot-oidc-issuer.md), will be used to publish the public OIDC
metadata discovery documents `/.well-known/openid-configuration` and `jwks_uri`.
This component will be only provided with access to the public keys or any other
public information, it will not hold or have access to any private information
related to token generation and signing. To support key rotation, it will serve
also the older set of public keys so that already issued but still valid and not
expired tokens can be used for identity federation with external services.

On key rotation, the new key pair might need to be published but not used to
sign the tokens, this is needed to ensure enough time for the external services
to discover the new public key. This rotation strategy could be useful for
external services that do not automatically rediscover the OIDC issuer metadata
when the token is signed with still unknown to them key. The major
infrastructure providers do not document publicly how often they are running
OIDC rediscovery, but a hands-on experience shows that some are doing it
immediately, while others need several minutes. As workload identity is not
limited only to the major infrastructure providers, therefore the duration of
this period will be configurable and it would be recommended to be at least one
day long.

The Kubernetes API server extended by the Gardener API server is already issuing
JWTs for the Kubernetes service accounts. To completely separate workload
identity JWTs from service accounts JWTs, Gardener API will accept an issuer URL
parameter whose value should not be the same as the issuer of the Kubernetes
service accounts. The workload identity issuer url should not be among the
accepted issuers of the Kubernetes API server. Other configuration options for
the Gardener API server will be the private key used to sign the tokens, the
minimal and maximal durations for each token. The private key also should not be
shared with the Kubernetes API server. When `gardener-operator` is used to
manage the Garden cluster, it will be also responsible for the Workload Identity
token signing key rotation, a strategy similar to the one for the Kubernetes
Service Account token signing key rotation will be used.

When Gardener API server is using own issuer and signing keys, the service
account token authenticator of the Kubernetes API server will reject the
workload identity JWTs because:

- the issuer of the tokens is not accepted
- the tokens are not signed by trusted key
- workload identity JWTs are not referring to any Kubernetes service account
- Gardener API will not serve the purpose of authentication or authorization
  webhook, it will also not implement any authentication or authorization based
  on the workload identity JWTs, it will just generate and sign them.

Gardener API server will use the global configurations, `WorkloadIdentity` and
`TokenRequest` specifications to issue JSON Web Tokens. Later, if a use case is
identified, it could feature custom claims in the `gardener.cloud` claim
namespace that contain additional information about the context of use of the
token, e.g. metadata about the shoot, seed, project, garden, etc.

A sample payload of a token will look like:

```json
{
    "aud": [
        "service-foo-provider-bar"
    ],
    "exp": 1707315742,
    "iat": 1707312142,
    "nbf": 1707312142,
    "iss": "https://workload-identity.gardener-local.gardener.cloud",
    "sub": "gardener.cloud:workloadidentity:<workloadidentity-namespace>:<workloadidentity-name>:<workloadidentity-uid>",
    "gardener.cloud": {
        "workloadIdentity": {
            "name": "<workloadidentity-name>",
            "namespace": "<workloadidentity-namespace>",
            "uid": "<workloadidentity-uid>",
        },
        "shoot": {
            "name": "<shoot-name>",
            "namespace": "<shoot-namespace>",
            "uid": "<shoot-uid>",
        },
        "project": {
            "name": "<project-name>",
            "uid": "<project-uid>",
        },
        "seed": {
            "name": "<seed-name>",
            "uid": "<seed-uid>",
        },
    }
}
```

### Distribution of Workload Identity Tokens

Gardenlet will request tokens as per the global configurations and renew them
regularly. It is expected gardenlet to not specify duration for the tokens using
the default token duration, but this can be changed later based on the actual
needs. It will be responsible to provide information for the specific usage of
the token, e.g. shoot name, namespace and UID, via the `TokenRequest` API.
`gardenlet` will be the only Gardener component authorized to create and refresh
workload identity tokens. Seed Authorizer will be extended to allow gardenlets
to request workload identity tokens only for `WorkloadIdentity` that they are
responsible for.

As the tokens will usually have lifetime shorter than the period between two
reconciliations, it is essential that the token creation and management are
decoupled from the current control loops of gardenlet and implemented by a
dedicated controller, also running as part of the gardenlet. Forced renewal of
the tokens will be performed when the resource referring `WorkloadIdentity`s is
annotated with `gardener.cloud/operation=renew-workload-identity-token`. The
annotation is deliberately not set on the `WorkloadIdentity` because single
`WorkloadIdentity` can be used by multiple shoots potentially running on
different seeds, i.e. multiple controllers would be responsible to react on the
annotation which is usually fine, but all of them would have to negotiate when
the operation is completed and the annotation to be removed.

Kubernetes Secrets will be the resource holding the JWT, the provider config,
and metadata about the used workload identity. The JWT will be stored under the
`token` data key, while the provider config will use the `config` data key. The
name and namespace of the used `WorkloadIdentity` will be stored in the
annotations `workloadidentity.security.gardener.cloud/name` and
`workloadidentity.security.gardener.cloud/namespace` respectively. The
annotation `workloadidentity.security.gardener.cloud/context-object`, if
present, will hold reference to the object using the workload identity, the
value will be JSON document and have the format
`{"apiVersion":"...","kind":"...","name":"...","namespace":"...","uid":"..."}`.
It will be the source for the `spec.contextObject` field of the `TokenRequest`.
To let the controller(s) easily select or distinguish these secrets, they will
be labeled with
`security.gardener.cloud/purpose: workload-identity-token-requestor`. The
secrets will be also labeled with
`workloadidentity.security.gardener.cloud/provider=<WorkloadIdentity.spec.targetSystem.type>`
so that the extensions can easily select them and make adjustments via admission
webhooks, e.g. transform the service provider config and the token into
canonical form usable by the respective service provider SDK.

A sample secret bearing workload identity token will look like:

```yaml
apiVersion: v1
kind: Secret
data:
  config: YXBpVmV...bmZpZw==
  token: eyJhbGciOiJ....OkBBrVWA
stringData:
  credentialsFile: | # Field injected by extension admission webhook, key name is controller by the extension
    [default]
    role_arn=arn:aws:iam::112233445566:role/gardener-dev
    web_identity_token_file=/var/run/workload-identity/aws/token
metadata:
  name: cloudprovider
  namespace: shoot--local--foo
  annotations:
    workloadidentity.security.gardener.cloud/namespace: garden-local
    workloadidentity.security.gardener.cloud/name: banana-testing
    workloadidentity.security.gardener.cloud/context-object: '{"apiVersion":"core.gardener.cloud/v1beta1","kind":"Shoot","name":"foo","namespace":"garden-local","uid":"54d09554-6a68-4f46-a23a-e3592385d820"}'
  labels:
    security.gardener.cloud/purpose: workload-identity-token-requestor
    workloadidentity.security.gardener.cloud/provider: aws # {aws,azure,gcp,...}
type: Opaque
```

The secret `cloudprovider` that now holds the static credentials will be reused
to store the token and the provider config when the shoot is using workload
identity as infrastructure credentials. For each extension using workload
identity, the secret will be named
`workloadidentity-extension-<extension-type>`, and for dns providers
`workloadidentity-dns-<dns-provider-type>`.

The reconciliation flow for a component using workload identity tokens will look
like this:

1. A gardenlet controller creates/updates a `secret` resource in the seed with
   the above mentioned annotations and labels based on the configuration of the
   `WorkloadIdentity` and resource using it. The workload identity provider
   config is also written into the secret at this step.
1. Optionally, extension webhook intercepts the `CREATE` or
   `UPDATE` request for the secret and makes extension specific adjustment to
   the secret.
1. The dedicated controller watches these secrets and receives event to
   reconcile it. The controller reads the current token from the secret and if
   it does not exist or is due for rotation, a new token is requested via
   `TokenRequest` on the respective `WorkloadIdentity/token` subresource. The
   controller writes the returned token into the secret.
1. Optionally, extension webhook intercepts the `UPDATE` request for the secret
   and makes extension specific adjustment to the secret.
1. The `secret` is requeued for reconciliation again when the token will be
   suitable for renewal.
1. Secret is mounted as volume to the pods that requires credentials to interact
   with the external service. Optionally, extension webhooks can change mount
   paths, mount additional keys from the secret, set environment variables, etc.

Just for completeness, here is how the flow will look like for cloud controller
manager when AWS shoot cluster is using workload identity:

1. The shoot controller from gardenlet creates the `cloudprovider` secret in the
   shoot namespace in the seed with the above described labels and annotations,
   also writing the workload identity `providerConfig` into the secret under the
   `config` key.
1. `gardener-extension-provider-aws` webhook intercepts the create request and
   injects `credentialsFile` data key which value is derived from the content of
   the `config` key. `credentialsFile` content is AWS profile config file
   containing the ARN of the IAM role to be assumed and path to the web identity
   token file.
1. The gardenlet controller dedicated to manage the tokens, reconciles the
   secret. A token is issued and written into the `token` key of the secret.
1. The `gardener-extension-provider-aws` webhook is triggered again and the same
   adjustments are made.
1. The `controlplane` controller from `gardener-extension-provider-aws` deploys
   the `cloud-controller-manager` with the required adjustments, e.g. mount the
   `token` key from the `cloudprovider` secret on path
   `/var/run/workload-identity/aws/token` and set the environment variables
   `AWS_ROLE_ARN=arn:aws:iam::112233445566:role/gardener-dev`, and
   `AWS_WEB_IDENTITY_TOKEN_FILE=/var/run/workload-identity/aws/token`, or use
   other options to configure the AWS SDK like `AWS_SHARED_CREDENTIALS_FILE`
   environment variable having the value of the `credentialsFile` key from the
   secret.

### Use cases

- Replaces static infrastructure credentials for shoot clusters running on AWS,
  Azure, Alibaba and GCP.
- Replaces static credentials used by backup controllers to interact with object
  storage service on AWS, Azure, Alibaba and GCP.
- Replaces static credentials used by dns controllers to interact with DNSaaS on
  AWS, Azure, Alibaba and GCP.
- Replaces static credentials used by certificate controllers to interact with
  CERTaaS providers.
- Others, for example Alertmanager receivers.
- Image pull secrets for private images.

## Alternatives

### SPIFFE/SPIRE

It should be possible to run SPIRE server in the Gardener landscape, presumably
in the runtime cluster, and SPIRE agents on each seed clusters.

However, this would come with decent overhead:

- Bootstrapping the system and managing the credentials for different agents
  will need to be automated.
- The solution could easily lock-in to the SPIFFE/SPIRE as identity issuer and
  make it hard to change the implementation, if needed.
- It is a 3rd party solution not so similar to Kubernetes that needs to be
  learned, operated and maintained.
- Limited flexibility to inject Gardener own data in the tokens. Eventually
  could be achieved with custom plugins.
- A standalone API endpoint that need to be properly operated, also all agents
  will need to be bootstrapped securely.
- SPIRE agents associate the identities with the nodes where the workload is
  running, but in Gardener we are more interested in the Seeds and the workload
  itself, not the nodes. With custom plugins it should be possible to make it
  fit the Gardener case.
- SPIRE server needs a database to store various information about the cluster,
  operating additional stateful component will require certain investment.
- Seems to be not so friendly when the server and agents are running in
  different clusters, especially on different clouds, as node attestation done
  by the server needs somehow to evaluate the nodes with the cloud providers.
  With custom plugins it should be possible to make it fit Gardener case.

### Kubernetes Service Account Tokens From Garden Cluster

Using a dedicated resource instead of the `ServiceAccount` from k8s core API is
preferred because of several reasons:

- Requesting a workload identity token should be accessible only by workloads
  running in the Gardener environment, i.e. they should not be exported and used
  by other tools, services, application, etc. Gardener users are already granted
  with access to create tokens for `ServiceAccounts` in their project namespace
  and this cannot be restricted without introducing breaking changes.
- To ensure that workload identity tokens cannot be used for authentication with
  the Gardener API. `WorkloadIdentity` is designed to provide authentication
  with external services and not with Gardener API.

### Kubernetes Service Account Tokens From Seed Cluster

A Gardener landscape is highly dynamic and seed clusters are added and removed
regularly. Also, shoot clusters are migrated between different seeds on demand.
Managing trust configuration toward multiple seeds (tens or even hundreds of
seeds), is cumbersome work, especially when the ones responsible for the trust
configurations are not responsible for the seeds.
