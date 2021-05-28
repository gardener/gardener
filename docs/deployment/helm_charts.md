# Helm charts

Helm charts are maintained in the `charts` directory.

To enable deployment directly via helm without depending on addons or a cloned git repository,
some charts are also published to GitHub pages.

You can add them with

```sh
helm repo add gardener https://gardener.github.io/gardener/
helm repo update
```

Currently, only the following charts are part of the chart repository:

* controlplane
* gardenlet
