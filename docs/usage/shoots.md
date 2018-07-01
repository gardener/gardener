# Creating / Deleting a Shoot cluster

As you have already prepared an example Shoot manifest in the steps described in the development documentation, please open another Terminal pane/window with the `KUBECONFIG` environment variable pointing to the Garden development cluster and send the manifest to the Kubernetes API server:

```bash
$ kubectl apply -f $GOPATH/src/github.com/gardener/gardener/dev/shoot-aws.yaml
```

You should see that the Gardener has immediately picked up your manifest and started to deploy the Shoot cluster.

In order to investigate what is happening in the Seed cluster, please download its proper Kubeconfig yourself (see next paragraph). The namespace of the Shoot cluster in the Seed cluster will look like that: `shoot-johndoe-johndoe-1`, whereas the first `johndoe` is your namespace in the Garden cluster (also called "project") and the `johndoe-1` suffix is the actual name of the Shoot cluster.

To connect to the newly created Shoot cluster, you must download its Kubeconfig as well. Please connect to the proper Seed cluster, navigate to the Shoot namespace, and download the Kubeconfig from the `kubecfg` secret in that namespace.

In order to delete your cluster, you have to set an annotation confirming the deletion first, and trigger the deletion after that. You can use the prepared `delete-shoot` script which takes the Shoot name as first parameter. The namespace can be specified by the second parameter, but it is optional. If you don't state it, it defaults to your namespace (the username you are logged in with to your machine).

```bash
$ ./hack/delete-shoot johndoe-1
```

# Updating Shoot Cluster version and How Auto Update Feature is Handled

If a shoot has `.spec.maintenance.autoUpdate.kubernetesVersion: true` in the manifest, and you update the `.spec.<provider>.constraints.kubernetes.versions` field in the CloudProfile used in the Shoot, then Gardener will apply Kubernetes [patch releases](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/release/versioning.md#patch-releases) updates automatically during the `.spec.maintenance.timeWindow`.

Since Kubernetes follows [Semantic Versioning](http://semver.org/), if indicated so, Gardener will automatically apply the patch release updates. But it will never auto update the Major or Minor releases since there is no effort to keep backward compatibility in those releases.

Major or Minor updates must be handled by updating the `.spec.kubernetes.version` field manually, theese updates will be executed immediately and will not wait for maintenance time window. **Before applying such update on Minor or Major releases, operators should check for all the breaking chances introduced in the target release Changelog**.

E.g. If you have a shoot cluster with below field values (only related fields are shown):

```
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

If you update the CloudProfile used in the Shoot and add `1.10.5` and `1.11.0` to the `.spec.<provider>.constraints.kubernetes.versions` list, the Shoot will be updated to `1.10.5` between 22:00-23:00 UTC. Your Shoot won't be updated to `1.11.0` even though its the highest Kubernetes in the CloudProfile, this is because that woulnd't be a patch release update but a minor release update, and potentially have breaking changes that could impact your deployed resources.

In this example if the operator wants to update the Kubernetes version to `1.11.0`, he/she must update the Shoot's `.spec.kubernetes.version` to `1.11.0` manually.
