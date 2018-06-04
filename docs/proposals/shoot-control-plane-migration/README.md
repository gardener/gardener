#Shoot Control-plane Migration

## Background

Gardener uses the seed-shoot architecture where the control-plane of multiple shoot clusters are hosted on a managed seed cluster. There can be many such seed clusters depending on the cloud provider, region etc.

## Motivation

The availability and health of the shoot control-planes is the responsibility of the corresponding seed cluster. But there can be many challenges to the task of the seed cluster to keep the shoot control-planes available and healthy. Some such challenges might be as below.

* The seed cluster might run out of resources to run the control-planes.
* The seed cluster's control-plane might become unavailable.
* Network between the shoot control-plane in the seed and the actual shoot cluster might become unavailable.
* The whole of the seed cluster might go down.

Such challenges might appear and last for different time periods ranging between transient to permanent.

There is also a future possibility of having a set of seed clusters serving a particular set of shoot clusters on a cloud provider, region etc. to address both reliability as well as load distribution among other issues.

Given such challenges and requirements, we need some mechanism to migrate/move a shoot cluster's control plane between seed clusters.

## Goal
* Provide a mechanism to migrate/move a shoot cluster's control-plane between seed clusters.
* The mechanism should be extensible in the future to support migration across regions/availability zones.
* The mechanism should be automatable.
* The mechanism should work regardless whether the source seed cluster and the shoot control-plane running there is available and healthy or not. I.e., the mechanism should work for the disaster recovery scenario as well as a scenarios such as seed load-balancing.

To be more explicit, the following scenarios should be supported.

| Scenario | Source Seed Reachable from Garden Cluster | Source Seed Running | Source Seed Can Reach Anything Outside |
| --------:| ----------------------------------------- | ------------------- | -------------------------------------- |
| 1        | Yes                                       | Yes/No              | Yes/No                                 |
| 2        | No                                        | Yes                 | Yes                                    |
| 3        | No                                        | No                  | No                                     |
| 4        | No                                        | Yes                 | No                                     |

## Non-goal
* The proposed mechanism need not actually implement a solution for migration across regions/availability zones.

## Reuse
* [Etcd Backup Restore](https://github.com/gardener/etcd-backup-restore) already backs up the shoot etcd which is running in the shoot namespace of the source seed cluster. We can re-use this completely while restoring the shoot etcd on the destination seed cluster.

## Proposed Solution

### Backup and Restoration

The backup and restoration of the following resources that are currently not stored in the shoot apiserver/etcd is not covered by the reuse mentioned above. Resources such as below.

* Resources from the shoot namespace in the seed.
  * Terraform resources such as configmaps and secrets.
  * Machine Controller Manager resources such as machinedeployments, machinesets, machines, machineclasses, secrets etc.
* Resources from the shoot backup infrastructure namespace in the seed.
  * Terraform resources such as configmaps and secrets.

All the additional resources listed will be backed up in the same object store as the what is used for backing up the shoot etcd.
* The backup will be watch-based and not schedule based.
* The backup will be full backups at longer intervals followed by incremental backups at smaller intervals similar to the etcd continuous/incremental backup.
    * But the granularity of the changes stored at the incremental backups would be at the object level and not the actual field-level changes. This is different from the etcd continuous/incremental backups where the incremental changes are recorded at the field level.

### Migration Co-ordination


## Possible Variations

## Alternatives