# Generic CSI Migration Controller

This package contains a generic CSI migration controller that helps provider extensions that are still using the legacy in-tree volume provisioner plugins to migrate to CSI.

## How does CSI (migration) work in general?

Classically, the `kube-controller-manager` was responsible for provisioning/deprovisioning and attaching/detaching persistent volumes.
The `kubelet` running on the worker nodes was mounting/unmounting the disks.
As there are so many vendor-specific implementations that were all maintained in the main Kubernetes source tree, the community decided to move them out into  dedicated drivers, and let the core Kubernetes components interact with them using a standardized interface (CSI).

In the Gardener context, we usually have two parts of the CSI components: One part contains the controllers that are to be deployed in the seed cluster (as part of shoot control plane).
They comprise the vendor-specific driver (e.g., AWS EBS driver), plus a generic provisioner, attacher, snapshotter, and resizer controller.
The other part is deployed as `DaemonSet` on each shoot worker node and is responsible for registering the CSI driver to the `kubelet` as well as mounting/unmounting the disks to the machines.

In order to tell the Kubernetes components that they are no longer responsible for volumes but CSI is used, the community has introduced feature gates.
There is a general `CSIMigration` feature gate, plus two vendor-specific feature gates, e.g. `CSIMigrationAWS` and `CSIMigrationAWSComplete`:

* If only the first two feature gates are enabled then CSI migration is in process. In this phase the `kube-controller-manager` / `kubelet`s are still partly responsible. Concretely, the `kube-controller-manager` will still provision/deprovision volumes using legacy storage class provisioners (e.g., `kubernetes.io/aws-ebs`), and it will attach/detach volumes for nodes that have not yet been migrated to CSI.
* If all three feature gates are enabled then CSI migration is completed and no in-tree plugin will be used anymore. Both `kube-controller-manager` and `kubelet` pass on responsibility to the CSI drivers and controllers.

For newly created clusters all feature gates can be enabled directly from the beginning and no migration is needed at all.

For existing clusters there are a few steps that must be performed.
Usually, the `kube-controller-manager` ran with the `--cloud-config` and `--external-cloud-volume-plugin` flags when using the in-tree volume provisioners.
Also, the `kube-apiserver` enables the `PersistentVolumeLabel` admission plugin, and the `kubelet` runs with `--cloud-provider=aws`.
As part of the CSI migration, the `CSIMigration<Provider>Complete` feature gate may only be enabled if all nodes have been drained, and all `kubelet`s have been updated with the feature gate flags.

As provider extensions usually perform a rolling update of worker machines when the Kubernetes minor version is upgraded, it is recommended to start the CSI migration during such a Kubernetes version upgrade.
Consequently, the migration can now happen by executing with the following steps:

1. The `CSIMigration` and `CSIMigrationAWS` feature gates must be enabled on all master components, i.e., `kube-apiserver`, `kube-controller-manager`, and `kube-scheduler`.
1. Until the CSI migration has completed the master components keep running with the cloud provider flags allowing to use in-tree volume provisioners.
1. A rolling update of the worker machines is triggered, new `kubelet`s are coming up with all three CSI migration feature gates enabled + the `--cloud-provider=external` flag.
1. After only new worker machines exist the master components can be updated with the `CSIMigration<Provider>Complete` feature gate + removal of all cloud-specific flags.
1. As `StorageClass`es  are immutable the existing ones using a legacy in-tree provisioner can be deleted so that they can be recreated with the same name but using the new CSI provisioner. The CSI drivers are ensuring that they stay compatible with the legacy provisioner names [forever](https://kubernetes.slack.com/archives/CG04EL876/p1578500027013800?thread_ts=1578474999.005000&cid=CG04EL876).  

## How can this controller be used for CSI migration?

As motivated in above paragraph, for provider extensions the easiest way to start the CSI migration is together with a Kubernetes minor version upgrade because the necessary rolling update of the worker machines is triggered.

The problem now is that the steps 4) and 5) must happen only after no old nodes exist in the cluster anymore.
This could be done in the `Worker` controller, however, it would be somewhat ugly and mix too many things together (it's already pretty large).
Having such dedicated CSI migration controller allows for better separation, maintainability and less complexity.

The idea is that a provider extension adds this generic CSI migration controller - similar how it adds other generic controllers (like `ControlPlane`, `Infrastructure`, etc.).
It will watch `Cluster` resources of shoots having the respective provider extension type + minimum Kubernetes version that was declared for starting CSI migration.
When it detects such a `Cluster` it will start its CSI migration flow.

The (soft) contract between the CSI migration controller and control plane webhooks of provider extensions is that the CSI migration controller will annotate the `Cluster` resource with `csi-migration.extensions.gardener.cloud/needs-complete-feature-gates=true` in case the migration is finished.
The control webhooks can read this annotation and - if present - configure the Kubernetes components accordingly (e.g., adding the `CSIMigration<Provider>Complete` feature gate + removing the cloud flags).

### CSI Migration Flow

1. Check if the shoot is newly created - if yes, annotate the `Cluster` object with `csi-migration.extensions.gardener.cloud/needs-complete-feature-gates=true` and exit.
1. If the cluster is an existing one that is getting updated then
   1. If the shoot is hibernated then requeue and wait until it gets woken up.
   1. Wait until only new nodes exist in the shoot anymore.
   1. Delete the legacy storage classes in the shoot.
   1. Add the `csi-migration.extensions.gardener.cloud/needs-complete-feature-gates=true` annotation to the `Cluster`.
   1. Send empty `PATCH` requests to the `kube-apiserver`, `kube-controller-manager`, `kube-scheduler` `Deployment` resources to allow the control plane webhook adapting the specification.

Consequently, from the provider extension point of view, what needs to be done is

1. Add the `CSIMigration` controller and start it.
1. Deploy CSI controller to seed and CSI driver as part of the `ControlPlane` reconciliation for shoots of the Kubernetes version that is used for CSI migration.
1. Add the `CSIMigration` and `CSIMigration<Provider>` feature gates to the Kubernetes master components (together with the cloud flags) if the `Cluster` was not yet annotated with `csi-migration.extensions.gardener.cloud/needs-complete-feature-gates=true`.
1. Add the `CSIMigration<Provider>Complete` feature gate and remove all cloud flags if the `Cluster` was annotated with `csi-migration.extensions.gardener.cloud/needs-complete-feature-gates=true`.

## Further References

* k8s.io blog about CSI Migration: https://kubernetes.io/blog/2019/12/09/kubernetes-1-17-feature-csi-migration-beta/
* KEP for CSI Migration: https://github.com/kubernetes/enhancements/blob/master/keps/sig-storage/20190129-csi-migration.md
