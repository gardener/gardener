# IPv6 in Gardener Clusters

> 🚧 IPv6 networking is currently under development.

## IPv6 Single-Stack Networking

[GEP-21](../proposals/21-ipv6-singlestack-local.md) proposes IPv6 Single-Stack Support in the local Gardener environment.
This documentation will be enhanced while implementing GEP-21, see [gardener/gardener#7051](https://github.com/gardener/gardener/issues/7051).

To use IPv6 single-stack networking, the [feature gate](../deployment/feature_gates.md) `IPv6SingleStack` must be enabled on gardener-apiserver and gardenlet.

## Development Setup

Developing or testing IPv6-related features requires a Linux machine (docker only supports IPv6 on Linux) and native IPv6 connectivity to the internet.
If you're on a different OS or don't have IPv6 connectivity in your office environment or via your home ISP, make sure to check out [gardener-community/dev-box-gcp](https://github.com/gardener-community/dev-box-gcp), which allows you to circumvent these limitations.

You can follow the guide on [Deploying Gardener Locally](../deployment/getting_started_locally.md) for setting up an IPv6 gardener for testing or development purposes.

## Container Images

If you plan on using custom images, make sure your registry supports IPv6 access.

The `docker.io` registry doesn't support pulling images over IPv6 (see [Beta IPv6 Support on Docker Hub Registry](https://www.docker.com/blog/beta-ipv6-support-on-docker-hub-registry/)).
There is a [prow job](https://github.com/gardener/ci-infra/blob/92782bedd92815639abf4dc14b2c484f77c6e57d/config/jobs/ci-infra/copy-images.yaml) copying images from Docker Hub that are needed in gardener components to the gardener GCR under the prefix `eu.gcr.io/gardener-project/3rd/` (see the [documentation](https://github.com/gardener/ci-infra/tree/master/config/images) or [gardener/ci-infra#619](https://github.com/gardener/ci-infra/issues/619)).
If you want to use a new image from Docker Hub or upgrade an already used image to a newer tag, please open a PR to the ci-infra repository that modifies the job's list of images to copy: [`images.yaml`](https://github.com/gardener/ci-infra/blob/master/config/images/images.yaml).
