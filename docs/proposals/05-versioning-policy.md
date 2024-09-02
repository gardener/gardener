# Gardener Versioning Policy

Please refer to [this document](../usage/shoot-operations/shoot_versions.md) for the documentation of the implementation of this GEP.

## Goal

- As a Garden operator, I would like to define a clear Kubernetes version policy, which informs my users about deprecated or expired Kubernetes versions.
- As a user of Gardener, I would like to get information which Kubernetes version is supported for how long. I want to be able to get this information via API (cloudprofile) and also in the Dashboard.

## Motivation

The Kubernetes community releases **minor** versions roughly every three months and usually maintains **three minor** versions (the actual and the last two) with bug fixes and security updates. Patch releases are done more frequently. Operators of Gardener should be able to define their own Kubernetes version policy. This GEP suggests the possibility for operators to classify Kubernetes versions while they are going through their "maintenance life-cycle".

## Kubernetes Version Classifications

An operator should be able to classify Kubernetes versions differently while they go through their "maintenance life-cycle", starting with **preview**, **supported**, **deprecated**, and finally **expired**. This information should be programmatically available in the `cloudprofiles` of the Garden cluster as well as in the Dashboard. Please also note, that Gardener keeps the control plane and the workers on the same Kubernetes version.

For further explanation of the possible classifications, we assume that an operator wants to support four minor versions, e.g. v1.16, v1.15, v1.14, and v1.13.

- **preview:** After a fresh release of a new Kubernetes **minor** version (e.g. v1.17.0), the operator could tag it as _preview_ until he has gained sufficient experience. It will not become the default in the Gardener Dashboard until he promotes that minor version to _supported_, which could happen a few weeks later with the first patch version.

- **supported:** The operator would tag the latest Kubernetes patch versions of the actual (if not still in _preview_) and the last three minor Kubernetes versions as _supported_ (e.g. v1.16.1, v1.15.4, v1.14.9, and v1.13.12). The latest of these becomes the default in the Gardener Dashboard (e.g. v1.16.1).

- **deprecated:** The operator could decide that he generally wants to classify every version that is not the latest patch version as _deprecated_ and flag this versions accordingly (e.g. v1.16.0 and older, v1.15.3 and older, 1.14.8 and older, as well as v1.13.11 and older). He could also tag all versions (latest or not) of every Kubernetes minor release that is neither the actual, nor one of the last three minor Kubernetes versions as _deprecated_, too (e.g. v1.12.x and older). Deprecated versions will eventually expire (i.e., be removed).

- **expired:** This state is a _logical_ state only. It doesn't have to be maintained in the `cloudprofile`. All cluster versions whose `expirationDate` as defined in the `cloudprofile` is expired are automatically in this _logical_ state. After that date has passed, users cannot create new clusters with that version anymore and any cluster that is on that version will be forcefully migrated in its next maintenance time window, even if the owner has opted out of automatic cluster updates! The forceful update will pick the latest patch version of the current minor Kubernetes version. If the cluster was already on that latest patch version and the latest patch version is also expired, it will continue with latest patch version of the **next minor Kubernetes version**, so **it will result in an update of a minor Kubernetes version, which is potentially harmful to your workload, so you should avoid that/plan ahead!** If that's expired as well, the update process repeats until a non-expired Kubernetes version is reached, so, **depending on the circumstances described above, it can happen that the cluster receives multiple consecutive minor Kubernetes version updates!**

To fulfill his specific versioning policy, the Garden operator should be able to classify his versions, as well as set the expiration date in the `cloudprofiles`. The user should see these classifiers, as well as the expiration date in the dashboard.

