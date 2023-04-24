# Supported Kubernetes Versions

Currently, Gardener supports the following Kubernetes versions:

## Garden Clusters

The minimum version of a garden cluster that can be used to run Gardener is **`1.20.x`**.

## Seed Clusters

The minimum version of a seed cluster that can be connected to Gardener is **`1.20.x`**.

## Shoot Clusters

Gardener itself is capable of spinning up clusters with Kubernetes versions **`1.20`** up to **`1.26`**.
However, the concrete versions that can be used for shoot clusters depend on the installed provider extension.
Consequently, please consult the documentation of your provider extension to see which Kubernetes versions are supported for shoot clusters.

> 👨🏼‍💻 Developers note: The [Adding Support For a New Kubernetes Version](../development/new-kubernetes-version.md) topic explains what needs to be done in order to add support for a new Kubernetes version.

## Support Timeline

The Kubernetes project maintains the most recent three minor releases and releases a new minor version every 4 months.
This means that a release has patch support for approximately 1 year.
See the official [Releases](https://kubernetes.io/releases/) topic for the official upstream information.

In the past, the Gardener project did not have a policy regarding the number of supported Kubernetes versions at the same time.
Beginning with 2023, a new policy has been introduced:

> The Gardener project supports the last four Kubernetes minor versions and drops support for the oldest minor version as soon as support for a new minor version has been introduced.

| Kubernetes Version | End of Life | Supported Since | Support Dropped After         |
|--------------------|-------------|-----------------|-------------------------------|
| 1.20               | 2022-02-28  | v1.15.0         | 2023-01-31                    |
| 1.21               | 2022-06-28  | v1.21.0         | 2023-02-28                    |
| 1.22               | 2022-10-28  | v1.31.0         | 2023-04-30                    |
| 1.23               | 2023-02-28  | v1.39.0         | 1.27 is supported (> 2023-04) |
| 1.24               | 2023-07-28  | v1.48.0         | 1.28 is supported (> 2023-08) |
| 1.25               | 2023-10-28  | v1.56.0         | 1.29 is supported (> 2023-12) |
| 1.26               | 2024-02-28  | v1.63.0         | 1.30 is supported (> 2024-04) |

The three versions 1.20, 1.21, 1.22 (which all are officially out of maintenance already) are handled specially to allow users to adapt to this new policy.
Beginning with 1.23, the support of the oldest version is dropped after the support of a new version was introduced.

> ⚠️ Note that this guideline only concerns the code of `gardener/gardener` and is not related to the versions offered in `CloudProfile`s.
> It is recommended to always only offer the last three minor versions with `supported` classification in `CloudProfile`s and deprecate the oldest version with an expiration date before a new minor Kubernetes version is released.
