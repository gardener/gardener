---
title: Access Restrictions
---

# Access Restrictions

Access restrictions can be configured in the `CloudProfile`, `Seed`, and `Shoot` APIs.
They can be used to implement access restrictions for seed and shoot clusters (e.g., if you want to ensure "EU access"-only or similar policies).

## `CloudProfile`

The `.spec.regions` list contains all regions that can be selected by `Shoot`s.
Operators can configure them with a list of access restrictions that apply for each region, for example:

```yaml
spec:
  regions:
  - name: europe-central-1
    accessRestrictions:
    - name: eu-access-only
  - name: us-west-1
```

This configuration means that `Shoot`s selecting the `europe-central-1` region **can** configure an `eu-access-only` access restriction.
`Shoot`s running in other regions cannot configure this access restriction in their specification.

## `Seed`

The `Seed` specification also allows to configure access restrictions that apply for this specific seed cluster, for example:

```yaml
spec:
  accessRestrictions:
  - name: eu-access-only
```

This configuration means that this seed cluster can host shoot clusters that also have the `eu-access-only` access restriction.
In addition, this seed cluster can also host shoot clusters without any access restrictions at all.

## `Shoot`

If the `CloudProfile` allows to configure access restrictions for the selected `.spec.region` in the `Shoot` (see [above](#cloudprofile)), then they can also be provided in the specification of the `Shoot`, for example:

```yaml
spec:
  region: europe-central-1
  accessRestrictions:
  - name: eu-access-only
#   options:
#     support.gardener.cloud/eu-access-for-cluster-addons: "false"
#     support.gardener.cloud/eu-access-for-cluster-nodes: "true"
```

In addition, it is possible to specify arbitrary options (key-value pairs) for the access restriction.
These options are not interpreted by Gardener, but can be helpful when evaluated by other tools (e.g., [`gardenctl`](https://github.com/gardener/gardenctl-v2) implements some of them).

Above configuration means that the `Shoot` shall only be accessible by operators in the EU.
When configured for

- a newly created `Shoot`, `gardener-scheduler` will automatically filter for `Seed`s also supporting this access restriction.
  All other `Seed`s are not considered for scheduling.
- an existing `Shoot`, `gardener-apiserver` will allow removing access restrictions, but adding them is only possible if the currently selected `Seed` supports them.
  If it does not support them, the `Shoot` must first be migrated to another eligible `Seed` before they can be added.
- an existing `Shoot` that is migrated, `gardener-apiserver` will only allow the migration in case the targeted `Seed` also supports the access restrictions configured on the `Shoot`.

> [!IMPORTANT]
> There is no technical enforcement of these access restrictions - they are purely informational.
> Hence, it is the responsibility of the operator to ensure that they enforce the configured access restrictions.
