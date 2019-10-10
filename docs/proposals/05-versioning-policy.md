# Gardener Versioning Policy

## Goal

- As a Garden operator I would like to define a clear Kubernetes version policy, which informs my users about deprecated or expired Kubernetes versions.
- As an user of Gardener, I would like to get information which Kubernetes version is supported for how long. I want to be able to get this information via API (cloudprofile) and also in the Dashboard.

## Motivation

The Kubernetes community releases **minor** versions roughly every three months and usually maintains **three minor** versions (the actual and the last two) with bug fixes and security updates. Patch releases are done more frequently. Gardener creates plain vanilla Kubernetes clusters provided by the community and therefore relies on the community to provide bug fixes and security updates. This means in particular, that we do **not** back-port bug fixes or security updates to older Kubernetes versions which are not maintained by the community anymore.

However we are aware that there is demand from our stakeholders to stay on a specific Kubernetes version for a longer period of time. For that reason, Gardener offers also older Kubernetes versions on a voluntary basis which are not maintained by the community anymore until we learn of issues (security vulnerabilities, bugs, operational effort, etc.) that make it "unbearable" to keep that version available. There are no hard rules here and this decision will be taken on a case by case basis by the Gardener team. The expiration process will follow clear rules, however (see below).

## Kubernetes Version Classifications

️️️️️️⚠️ **_NOTE:_**
If you opt-out of automatic cluster updates, it is the responsibility of the cluster owner to update the clusters in case of known security vulnerabilities.

Gardener classifies Kubernetes versions differently while they go through their "maintenance life-cycle", starting with **preview**, **supported**, **deprecated**, and finally **expired**. This information is programmatically available in the `cloudprofiles` in the Garden cluster. Please also note, that Gardener keeps the control plane and the workers in lock-step, i.e. on the same Kubernetes version.

Example: Assuming the Kubernetes community maintains at present the following minor versions v1.16.1, v1.15.4 and v1.14.9, then we classify as follows:

- **preview:** After a fresh release of a new Kubernetes **minor** version (e.g. v1.16.0) we tag it as _preview_ until we have gained sufficient experience. It will not become the default in the Gardener Dashboard until we promote that minor version to _supported_, which usually happens a few weeks later with the first patch version (as a rule of thumb, e.g. v1.16.1, but it could be also v1.16.0 if it was stable and there was no patch version or v1.16.2 or higher if there were many because quality was not good enough in the beginning of that Kubernetes minor release).

- **supported:** We tag the latest Kubernetes patch versions of the actual (if not still in _preview_) and the last two minor Kubernetes versions as _supported_ (e.g. v1.16.1, v1.15.4 and v1.14.9). The latest of these becomes the default in the Gardener Dashboard (e.g. v1.16.1).

- **deprecated:** We generally tag every version that is not the latest patch version as _deprecated_ (e.g. v1.16.0, v1.15.3 and older, and v1.14.8 and older). We also tag all versions (latest or not) of every Kubernetes minor release that is neither the actual nor one of the last two minor Kubernetes versions as _deprecated_, too (e.g. v1.13.x and older). Deprecated versions will eventually expire (i.e. removed) following these rules:
  - We will generally let the non-latest (_deprecated_) Kubernetes patch versions of minor versions that are **still in maintenance** (the actual and the last two) expire with **4 months of head start** (e.g. if v1.14.8 needs to expire today, it will expire today + 4 months). This gives cluster owners that only have a quarterly maintenance downtime (should they not be able to run zero-downtime cluster updates in production, e.g. because they are running pet use cases or singletons that are down when migrated from one cluster node to another) sufficient time to validate and execute their cluster updates in advance.
  - We will generally let the non-latest (_deprecated_) Kubernetes patch versions of minor versions that have **run out of maintenance** expire with **1 month of head start** (e.g. if v1.13.x needs to expire today, it will expire today + 1 month). This gives cluster owners who have neglected their cluster updates a "fair reaction time". If they want a longer head start, they need to stay on Kubernetes minor versions that are in maintenance (see above).
  - As described above, the (final and) latest Kubernetes patch version of minor versions that have run out of maintenance expire when the Gardener team assesses it as "unbearable" to keep that Kubernetes minor version available. It will also expire with **1 month of head start** (like above) and then this Kubernetes minor version will be gone completely.

- **expired:** A Kubernetes version is expired when its `expirationDate` as per the `cloudprofile` is reached. After that date has passed, users cannot create new clusters with that version anymore and any cluster that is on that version will be forcefully migrated in its next maintenance time window, even if the owner has opted out of automatic cluster updates! The **forceful update** will pick the latest patch version of the next higher minor Kubernetes version. If that's expired as well, the update process repeats until a non-expired Kubernetes version is reached.
