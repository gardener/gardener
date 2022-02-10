# Authentication of Gardener control plane components against the Garden cluster

**Note:** This document refers to Gardener's API server, admission controller, controller manager and scheduler components. Any reference to the term **Gardener control plane component** can be replaced with any of the mentioned above.

There are several authentication possibilities depending on whether or not [the concept of *Virtual Garden*](https://github.com/gardener/garden-setup#concept-the-virtual-cluster) is used.

## *Virtual Garden* is not used, i.e., the `runtime` Garden cluster is also the `target` Garden cluster.

**Automounted Service Account Token**
The easiest way to deploy a **Gardener control plane component** will be to not provide `kubeconfig` at all. This way in-cluster configuration and an automounted service account token will be used. The drawback of this approach is that the automounted token will not be automatically rotated.

**Service Account Token Volume Projection**
Another solution will be to use [Service Account Token Volume Projection](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-token-volume-projection) combined with a `kubeconfig` referencing a token file (see example below).
```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: <CA-DATA>
    server: https://default.kubernetes.svc.cluster.local
  name: garden
contexts:
- context:
    cluster: garden
    user: garden
  name: garden
current-context: garden
users:
- name: garden
  user:
    tokenFile: /var/run/secrets/projected/serviceaccount/token
```

This will allow for automatic rotation of the service account token by the `kubelet`. The configuration can be achieved by setting both `.Values.global.GardenerControlPlaneComponent.serviceAccountTokenVolumeProjection.enabled: true` and `.Values.global.GardenerControlPlaneComponent.kubeconfig` in the respective chart's `values.yaml` file.

## *Virtual Garden* is used, i.e., the `runtime` Garden cluster is different from the `target` Garden cluster.

**Service Account**
The easiest way to setup the authentication will be to create a service account and the respective roles will be bound to this service account in the `target` cluster. Then use the generated service account token and craft a `kubeconfig` which will be used by the workload in the `runtime` cluster. This approach does not provide a solution for the rotation of the service account token. However, this setup can be achieved by setting `.Values.global.deployment.virtualGarden.enabled: true` and following these steps:

1. Deploy the `application` part of the charts in the `target` cluster.
2. Get the service account token and craft the `kubeconfig`.
3. Set the crafted `kubeconfig` and deploy the `runtime` part of the charts in the `runtime` cluster.

**Client Certificate**
Another solution will be to bind the roles in the `target` cluster to a `User` subject instead of a service account and use a client certificate for authentication. This approach does not provide a solution for the client certificate rotation. However, this setup can be achieved by setting both `.Values.global.deployment.virtualGarden.enabled: true` and `.Values.global.deployment.virtualGarden.GardenerControlPlaneComponent.user.name`, then following these steps:

1. Generate a client certificate for the `target` cluster for the respective user.
2. Deploy the `application` part of the charts in the `target` cluster.
3. Craft a `kubeconfig` using the already generated client certificate.
4. Set the crafted `kubeconfig` and deploy the `runtime` part of the charts in the `runtime` cluster.

**Projected Service Account Token**
This approach requires an already deployed and configured [oidc-webhook-authenticator](https://github.com/gardener/oidc-webhook-authenticator) for the `target` cluster. Also the `runtime` cluster should be registered as a trusted identity provider in the `target` cluster. Then projected service accounts tokens from the `runtime` cluster can be used to authenticate against the `target` cluster. The needed steps are as follows:

1. Deploy [OWA](https://github.com/gardener/oidc-webhook-authenticator) and establish the needed trust.
2. Set `.Values.global.deployment.virtualGarden.enabled: true` and `.Values.global.deployment.virtualGarden.GardenerControlPlaneComponent.user.name`. **Note:** username value will depend on the trust configuration, e.g., `<prefix>:system:serviceaccount:<namespace>:<serviceaccount>`
3. Set `.Values.global.GardenerControlPlaneComponent.serviceAccountTokenVolumeProjection.enabled: true` and `.Values.global.GardenerControlPlaneComponent.serviceAccountTokenVolumeProjection.audience`. **Note:** audience value will depend on the trust configuration, e.g., `<cliend-id-from-trust-config>`.
4. Craft a kubeconfig (see example below).
5. Deploy the `application` part of the charts in the `target` cluster.
6. Deploy the `runtime` part of the charts in the `runtime` cluster.

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: <CA-DATA>
    server: https://virtual-garden.api
  name: virtual-garden
contexts:
- context:
    cluster: virtual-garden
    user: virtual-garden
  name: virtual-garden
current-context: virtual-garden
users:
- name: virtual-garden
  user:
    tokenFile: /var/run/secrets/projected/serviceaccount/token
```
