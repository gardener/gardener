# Migration from Gardener `v0` to `v1`

With the release of Gardener `v1` the legacy API group `garden.sapcloud.io` has been entirely removed in favor of the new `core.gardener.cloud` group.

Also, the prefix that the Gardener `v0` API server was using to store its data in etcd was called `/registry/garden.sapcloud.io`.
This caused that the keys for resources looked for example like this: `/registry/garden.sapcloud.io/garden.sapcloud.io/shoots/garden-dev/foo`.

Now that Gardener `v1` only uses the new API group, it tries won't find the existing resources in etcd if we don't migrate their keys.
Also, we are using this situation in order to cleanup the prefix and get rid of the duplicate `garden.sapcloud.io` occurrences in the keys.

We are providing a general [etcd prefix migrator tool](../../cmd/registry-migrator/README.md) that can be used to copy old keys to new keys with a different prefix.

For the migration to Gardener `v1`, we are migrating the prefixes in two steps:

1. Migrate the `/registry/garden.sapcloud.io` prefix to `/registry-gardener` (that means that existing keys like `/registry/garden.sapcloud.io/garden.sapcloud.io/shoots/garden-dev/foo` will be copied to `/registry-gardener/garden.sapcloud.io/shoots/garden-dev/foo`).
1. Migrate the `/registry-gardener/garden.sapcloud.io` prefix to `/registry-gardener/core.gardener.cloud` (that means that existing keys like `/registry-gardener/garden.sapcloud.io/shoots/garden-dev/foo` will be copied to `/registry-gardener/core.gardener.cloud/shoots/garden-dev/foo`).

After that, the Gardener v1 API server is capable of using the old/existing data.
Newly created resources will be directly stored with the `/registry-gardener` prefix.

## ⚠️ Concrete steps (MUST FOLLOW!)

The following steps assume that you have a Gardener `v0` running, ideally Gardener `v0.35.x`.

* Make sure that you and your end-users don't use the `garden.sapcloud.io` API group anywhere.
* Perform an update on all resources that were in the `garden.sapcloud.io` API group to ensure they are written with the `core.gardener.cloud` version in etcd:
  ```shell
  kubectl get cloudprofiles,projects,quotas,seeds,secretbindings,shoots --all-namespaces -o json | kubectl replace -f -
  ```
* Scale down the replicas of your running Gardener `v0` API server pods to `0`.
* Wait until no API server that is talking to this etcd is running anymore.
* Either run the registry-migrator locally or via the provided Kubernetes `Job` (find the example manifest [here](../../cmd/registry-migrator/job-example.yaml)).
  * Locally:

    ```shell
    go run cmd/registry-migrator/main.go \
      --backup-file <path-to-file-for-step1> \
        --old-registry-prefix=/registry/garden.sapcloud.io \
        --new-registry-prefix=/registry-gardener \
        --endpoints <your-etcd-endpoint> \
        --ca <path-to-ca-file> \
        --cert <path-to-client-cert-file> \
        --key <path-to-client-key-file> \
        --force=false \
        --delete=false

    go run cmd/registry-migrator/main.go \
      --backup-file <path-to-file-for-step2> \
        --old-registry-prefix=/registry-gardener/garden.sapcloud.io \
        --new-registry-prefix=/registry-gardener/core.gardener.cloud \
        --endpoints <your-etcd-endpoint> \
        --ca <path-to-ca-file> \
        --cert <path-to-client-cert-file> \
        --key <path-to-client-key-file> \
        --force=false \
        --delete=false
    ```

  * Via Kubernetes `Job`:
    * Copy the [example manifest](../../cmd/registry-migrator/job-example.yaml) and insert the values at all places where you find a "TODO".
    * Apply the `Job` manifest via `kubectl apply`.
    * Wait until the `Job` is marked as `Completed`.
* Deploy the Gardener `v1` Helm chart.
* Scale up the replicas of your Gardener API server pods to your previous replica value.

After these steps, everything should behave normally.
The controller-manager, scheduler, and the gardenlets will reconnect to the API server after the scale up.

⚠️ Please note that you have to delete the old keys (with prefix `/registry/garden.sapcloud.io`) yourself after you have ensured that the migration worked correctly:

```shell
etcdctl del --prefix /registry/garden.sapcloud.io
```
