# Gardener Admission Controller

While the Gardener API server works with [admission plugins](./apiserver_admission_plugins.md) to validate and mutate resources belonging to Gardener related API groups, e.g. `core.gardener.cloud`, the same is needed for resources belonging to non-Gardener API groups as well.
Therefore, the Gardener Admission Controller runs a http(s) server with the following handlers which serve as validating/mutating endpoints for [admission webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/).

### Kubeconfig Secret Validator

[Malicious Kubeconfigs](https://github.com/kubernetes/kubectl/issues/697) applied by end users may cause a leakage of sensitive data.
This handler checks if the incoming request contains a Kubernetes secret with a `.data.kubeconfig` field and denies the request if the Kubeconfig structure violates Gardener's security standards.

### Namespace Validator

Namespaces are the backing entities of Gardener projects in which shoot clusters objects reside.
This validation handler protects active namespaces against premature deletion requests.
Therefore, it denies deletion requests if a namespace still contains shoot clusters or if it belongs to a non-deleting Gardener project (w/o `.metadata.deletionTimestamp`).