# Deploying the Gardener into a Kubernetes cluster

Similar to Kubernetes, Gardener consists out of control plane components (Gardener API server, Gardener controller manager, Gardener scheduler), and an agent component (Gardenlet).
The control plane is deployed in the so-called garden cluster while the agent is installed into every seed cluster.
Please note that it is possible to use the garden cluster as seed cluster by simply deploying the Gardenlet into it.

We are providing [Helm charts](../../charts/gardener) in order to manage the various resources of the components.
Please always make sure that you use the Helm chart version that matches the Gardener version you want to deploy.

## Deploying the Gardener control plane (API server, controller manager, scheduler)

The [configuration values](../../charts/gardener/controlplane/values.yaml) depict the various options to configure the different components.
Please consult [this document](../usage/configuration.md) to get a detailed explanation of what can be configured for which component.
Also note that all resources and deployments need to be created in the `garden` namespace (not overrideable).

After preparing your values in a separate `controlplane-values.yaml` file, you can run the following command against your garden cluster:

```bash
helm install charts/gardener/controlplane \
  --namespace garden \
  --name gardener-controlplane \
  -f gardener-values.yaml \
  --wait
```

## Deploying Gardener extensions

Gardener is an extensible system that does not contain the logic for provider-specific things like DNS management, cloud infrastructures, network plugins, operating system configs, and many more.

You have to install extension controllers for these parts.
Please consult [the documentation regarding extensions](../extensions/overview.md) to get more information.

## Deploying the Gardener agent (Gardenlet)

The Gardenlet requires a bootstrap token as well as a bootstrap kubeconfig in order to properly register itself with the Gardener control plane.

The [configuration values](../../charts/gardener/gardenlet/values.yaml) depict the various options to configure it.
Please consult [this document](../concepts/gardenlet.md#component-configuration) to get a detailed explanation of what can be configured.

Prepare your values in a separate `gardenlet-values.yaml` file:

1. Create a bootstrap token secret in the `kube-system` namespace of the garden cluster (see [this](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/) and [this](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping/#bootstrap-tokens)).
1. Create a bootstrap kubeconfig containing this token:

```yaml
apiVersion: v1
kind: Config
current-context: gardenlet-bootstrap@default
clusters:
- cluster:
    certificate-authority-data: <ca-of-garden-cluster>
    server: https://<endpoint-of-garden-cluster>
  name: default
contexts:
- context:
    cluster: default
    user: gardenlet-bootstrap
  name: gardenlet-bootstrap@default
users:
- name: gardenlet-bootstrap
  user:
    token: <bootstrap-token>
```

3. Provide this bootstrap kubeconfig together with a desired name and namespace to the Gardenlet Helm chart values [here](../../charts/gardener/gardenlet/values.yaml#L31-L35):

```yaml
gardenClientConnection:
  bootstrapKubeconfig:
    name: gardenlet-kubeconfig-bootstrap
    namespace: garden
    kubeconfig: |
      <bootstrap-kubeconfig>
```

4. Define a name and namespace where the Gardenlet shall store the real kubeconfig it creates during the bootstrap process [here](../../charts/gardener/gardenlet/values.yaml#L31-L35):

```yaml
gardenClientConnection:
  kubeconfigSecret:
    name: gardenlet-kubeconfig
    namespace: garden
```

5. Define either `seedSelector` or `seedConfig` (see [this document](../concepts/gardenlet.md#seed-config-vs-seed-selector)

Now you are ready to deploy the Helm chart:

```bash
helm install charts/gardener/gardenlet \
  --namespace garden \
  --name gardenlet \
  -f gardenlet-values.yaml \
  --wait
```

:warning: A current prerequisite of Kubernetes clusters that are used as seeds is to have a pre-deployed `nginx-ingress-controller` to make the Gardener work properly.
Moreover, there should exist a DNS record `*.ingress.<SEED-CLUSTER-DOMAIN>` where `<SEED-CLUSTER-DOMAIN>` is the value of the `.dns.ingressDomain` field of [a Seed cluster resource](../../example/50-seed.yaml) (or the [respective Gardenlet configuration](../../example/20-componentconfig-gardenlet.yaml#L84-L85)).
