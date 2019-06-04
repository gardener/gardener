# Creating a Shoot Cluster

As you have already prepared an example Shoot manifest in the steps described in the development documentation, please open another Terminal pane/window with the `KUBECONFIG` environment variable pointing to the Garden development cluster and send the manifest to the Kubernetes API server:

```bash
$ kubectl apply -f $GOPATH/src/github.com/gardener/gardener/dev/shoot-aws.yaml
```

You should see that the Gardener has immediately picked up your manifest and started to deploy the Shoot cluster.

In order to investigate what is happening in the Seed cluster, please download its proper Kubeconfig yourself (see next paragraph). The namespace of the Shoot cluster in the Seed cluster will look like that: `shoot-johndoe-johndoe-1`, whereas the first `johndoe` is your namespace in the Garden cluster (also called "project") and the `johndoe-1` suffix is the actual name of the Shoot cluster.

To connect to the newly created Shoot cluster, you must download its Kubeconfig as well. Please connect to the proper Seed cluster, navigate to the Shoot namespace, and download the Kubeconfig from the `kubecfg` secret in that namespace.

# Deleting a Shoot Cluster

In order to delete your cluster, you have to set an annotation confirming the deletion first, and trigger the deletion after that. You can use the prepared `delete shoot` script which takes the Shoot name as first parameter. The namespace can be specified by the second parameter, but it is optional. If you don't state it, it defaults to your namespace (the username you are logged in with to your machine).

```bash
$ ./hack/delete shoot johndoe-1 johndoe
```

# Maintaining Shoot Cluster

Day two operations like updating [Kubernetes patch version](#kubernetes-patch-version) (if the auto-update is enabled) and updating [Operating system version](#operating-system-version) happen in the maintenance time window of the Shoot cluster. The maintenance time window is part of the shoot spec (`.spec.maintenance.timeWindow`). If it is not specified during Shoot creation, Gardener will default to a randomized time window (to spread the load). The time interval cannot be less than 30 minutes and more than 6 hours.

To trigger the maintenance operation, you can annotate the Shoot with `shoot.garden.sapcloud.io/operation: maintain`.

### Kubernetes Patch Version

If a Shoot has `.spec.maintenance.autoUpdate.kubernetesVersion: true` in the manifest, and you update the `.spec.<provider>.constraints.kubernetes.versions` field in the CloudProfile used in the Shoot, then the maintenance controller will apply Kubernetes [patch releases](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/release/versioning.md#patch-releases) updates automatically during the maintenance time window.

Since Kubernetes follows [Semantic Versioning](http://semver.org/), if indicated so, Gardener will automatically apply the patch release updates. But it will never auto update the Major or Minor releases since there is no effort to keep backward compatibility in those releases.

Major or Minor updates must be handled by updating the `.spec.kubernetes.version` field manually, these updates will be executed immediately and will not wait for maintenance time window. **Before applying such update on Minor or Major releases, operators should check for all the breaking changes introduced in the target release Changelog**.

E.g. If you have a Shoot cluster with the field values below (only related fields are shown):

```yaml
spec:
  kubernetes:
    version: 1.10.0
  maintenance:
    timeWindow:
      begin: 220000+0000
      end: 230000+0000
    autoUpdate:
      kubernetesVersion: true
```

If you update the CloudProfile used in the Shoot and add `1.10.5` and `1.11.0` to the `.spec.<provider>.constraints.kubernetes.versions` list, the Shoot will be updated to `1.10.5` between 22:00-23:00 UTC. Your Shoot won't be updated to `1.11.0` even though its the highest Kubernetes in the CloudProfile, this is because that wouldn't be a patch release update but a minor release update, and potentially have breaking changes that could impact your deployed resources.

In this example if the operator wants to update the Kubernetes version to `1.11.0`, he/she must update the Shoot's `.spec.kubernetes.version` to `1.11.0` manually.

### Operating System Version

If you update the `.spec.<provider>.constraints.machineImages` field in the CloudProfile used in the Shoot, then the maintenance controller will apply the new machine image to the Shoot spec (and will mark the Shoot to be reconciled) during the maintenance time window. During the reconciliation the corresponding `<Provider>MachineClass` resource in the Shoot namespace in the Seed will be updated and the machine controller manager will take care of the actual state to match the desired one.

# Shoot annotations

- `shoot.garden.sapcloud.io/ignore: true` 

  Indicates that the Shoot will never reach the  reconciliation flow (independent of the operation (create/update/delete)). Also `controllers.shoot.respectSyncPeriodOverwrite` property of Gardener controller manager needs to be `true`, otherwise the ignore annotation won't be respected.

- `shoot.garden.sapcloud.io/operation: <operation>` with allowed operations:

  - `reconcile` - indicates that a Shoot reconciliation shall be triggered.
  - `retry` - indicates that a failed Shoot reconciliation shall be retried.
  - `maintain` - indicates that the Shoot maintenance shall be executed as soon as possible.

# Configure a Shoot cluster alert receiver

The receiver of the Shoot alerts can be configured by adding the annotation `garden.sapcloud.io/operatedBy` to the Shoot resource. The value of the annotation has to be a valid mail address.

The alerting for the Shoot clusters is handled by the Prometheus Alertmanager. The Alertmanager will be deployed next to the control plane when the `Shoot` resource is annotated with the `garden.sapcloud.io/operatedBy` annotation and if a [SMTP secret](../deployment/configuration.md) exists.

If the annotation gets removed then the Alertmanager will be also removed during the next reconcilation of the cluster. The opposite is also valid if the annotation is added to an existing cluster.
