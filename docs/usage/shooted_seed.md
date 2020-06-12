# Shooted Seeds

Create shooted Seeds with the `shoot.gardener.cloud/use-as-seed` annotation

## Preparation

### Edit `garden` namespace

The namespace "garden" needs 2 new labels for the project:

```yaml
labels:
  gardener.cloud/role: project
  project.gardener.cloud/name: garden
```

### `garden` Project

The annotation works only for `Shoot`s created in the `garden` namespace. Consequently, we have to create a project for the `garden` namespace first using `kubectl` (we don't use the Gardener Dashboard as it would add a `garden` prefix for the namespace).

Example: [/example/05-project-dev.yaml](../../example/05-project-dev.yaml)

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Project
metadata:
  name: garden
spec:
  owner:
...
  namespace: garden
```

## Create Shoot

### Deploy Shoot

Keep in mind that the networks from the Seed and its Shoots have to be different. To create Shoots with the dashboard you have to set a different worker CIDR in the shooted Seed (spec.provider.infrastructureConfig and spec.networking.nodes) and set the `shootDefaults` in the `shoot.gardener.cloud/use-as-seed` annotaion to different CIDRs.

Optional: To use the same networks like on the garden/root cluster for the future Shoots, we can set different CIDRs for pods and services in the shooted Seed (spec.networking.pods and spec.networking.services)
Full example: [/example/90-shoot.yaml](../../example/90-shoot.yaml)

Set the following annotation on the Shoot to mark it as a shooted Seed.
Example:

```yaml
  annotations:
    shoot.gardener.cloud/use-as-seed: >-
      true,shootDefaults.pods=100.96.0.0/11,shootDefaults.services=100.64.0.0/13,disable-capacity-reservation,with-secret-ref
```

This annotation contains a few configurations for the Seed:
Option | Description
--- | ---
`true` | deploys the gardenlet into the Shoot which will automatically register the cluster as Seed
`no-gardenlet` | prevents the automatic deployment of the gardenlet into the Shoot. Instead, the `Seed` object will be created with the assumption that another gardenlet will be responsible for managing it (according to its `seedSelector` configuration).
`disable-capacity-reservation` | set `spec.settings.excessCapacity.enabled` in the Seed to false (see [/example/50-seed.yaml](../../example/50-seed.yaml))
`invisible` | set `spec.settings.scheduling.visible` in the Seed to false  (see [/example/50-seed.yaml](../../example/50-seed.yaml))
`visible` | set `spec.settings.scheduling.visible` in the Seed to true  (see [/example/50-seed.yaml](../../example/50-seed.yaml)) **default**
`disable-dns` | set `spec.settings.shootDNS.enabled` in the Seed to false  (see [/example/50-seed.yaml](../../example/50-seed.yaml))
`protected` | only Shoots in the `garden` namespace can use this Seed
`unprotected` | Shoots from all namespaces can use this Seed **default**
`with-secret-ref` | creates a secret with the kubeconfig of the cluster in the `garden` namespace in the garden cluster and specifies the `.spec.secretRef` in the `Seed` object accordingly.
`shootDefaults.pods` | default pod network CIDR for Shoots created on this Seed
`shootDefaults.services` | default service network CIDR for Shoots created on this Seed
`minimumVolumeSize` | set `spec.volume.minimumSize` in the Seed (see [/example/50-seed.yaml](../../example/50-seed.yaml))
`blockCIDRs` | set `spec.network.blockCIDRs` seperated by `;` (see [/example/50-seed.yaml](../../example/50-seed.yaml))
`backup.provider` | set `spec.backup.provider` in the Seed (see [/example/50-seed.yaml](../../example/50-seed.yaml))
`backup.region` | set `spec.backup.region` in the Seed (see [/example/50-seed.yaml](../../example/50-seed.yaml))
`backup.secretRef.name` | set `spec.backup.secretRef.name` in the Seed (see [/example/50-seed.yaml](../../example/50-seed.yaml))
`backup.secretRef.namespace` | set `spec.backup.secretRef.namespace` in the Seed (see [/example/50-seed.yaml](../../example/50-seed.yaml))
`apiServer.autoscaler.minReplicas` | controls the minimum number of kube-apiserver replicas for the shooted Seed
`apiServer.autoscaler.maxReplicas` | controls the maximum number of kube-apiserver replicas for the shooted Seed
`apiServer.replicas` | controls how many kube-apiserver replicas the shooted Seed gets by default
`use-serviceaccount-bootstrapping` | states that the gardenlet shall register with the garden cluster using a temporary ServiceAccount instead of a CertificateSigningRequest (default)
