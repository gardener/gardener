# Creating a new Feature
If you want to contribute to the Gardener, please do that always on a dedicated branch on your own fork named after the purpose of the code changes, for example `feature/helm-integration`.
Please do not forget to rebase your branch **regularly**.

If you have finished your work, please [create a pull request](compare) **based on `master`**. It will be reviewed and merged if no further changes are requested from you.

:warning: Please ensure that your modifications pass the lint checks, formatting checks, static code checks, and unit tests by executing

:rotating_light: Please run `./hack/generate-code` whenever you modify the any API within `pkg/apis`.

```
$ make verify
```
Please do not file your pull request unless you receive a successful response from here!

# Creating a new Release or a Hotfix
Please refer to the [Gardener contributor guide](https://github.com/gardener/documentation/blob/master/CONTRIBUTING.md).
