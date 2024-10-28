# IPv6 in Gardener Clusters

> 🚧 IPv6 networking is currently under development.

## IPv6 Single-Stack Networking

[GEP-21](../proposals/21-ipv6-singlestack-local.md) proposes IPv6 Single-Stack Support in the local Gardener environment.
This documentation will be enhanced while implementing GEP-21, see [gardener/gardener#7051](https://github.com/gardener/gardener/issues/7051).

For real infrastructure providers, please check the corresponding provider documentation for IPv6 support.
Furthermore, please check the documentation of your preferred networking extension for IPv6 support.

## Development/Testing Setup

Developing or testing IPv6-related features requires a Linux machine (docker only supports IPv6 on Linux) and native IPv6 connectivity to the internet.
If you're on a different OS or don't have IPv6 connectivity in your office environment or via your home ISP, make sure to check out [gardener-community/dev-box-gcp](https://github.com/gardener-community/dev-box-gcp), which allows you to circumvent these limitations.

To get started with the IPv6 setup and create a local IPv6 single-stack shoot cluster, run the following commands:

```bash
make kind-up gardener-up IPFAMILY=ipv6
k apply -f example/provider-local/shoot-ipv6.yaml
```

Please also take a look at the guide on [Deploying Gardener Locally](../deployment/getting_started_locally.md) for more details on setting up an IPv6 gardener for testing or development purposes.

## Container Images

If you plan on using custom images, make sure your registry supports IPv6 access.

Check the [component checklist](../development/component-checklist.md#images) for tips concerning container registries and how to handle their IPv6 support.
