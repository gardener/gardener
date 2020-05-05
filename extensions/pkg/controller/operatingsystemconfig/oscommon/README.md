# [Gardener Extension for OS Configurations](https://gardener.cloud)

[![Go Report Card](https://goreportcard.com/badge/github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon)](https://goreportcard.com/report/github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon)

Project Gardener implements the automated management and operation of [Kubernetes](https://kubernetes.io/) clusters as a service. Its main principle is to leverage Kubernetes concepts for all of its tasks.

Recently, most of the vendor specific logic has been developed [in-tree](https://github.com/gardener/gardener). However, the project has grown to a size where it is very hard to extend, maintain, and test. With [GEP-1](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md) we have proposed how the architecture can be changed in a way to support external controllers that contain their very own vendor specifics. This way, we can keep Gardener core clean and independent.

The `oscommon` offers a generic controller that operates on the `OperatingSystemConfig` resource in the `extensions.gardener.cloud/v1alpha1` API group. It manages those objects that are requesting for an specific operating system. 


```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: OperatingSystemConfig
metadata:
  name: pool-01-original
  namespace: default
spec:
  type: <os type>
  units:
    ...
  files:
    ...
```

Please find [a concrete example](example/operatingsystemconfig.yaml) in the `example` folder.

After reconciliation the resulting data will be stored in a secret within the same namespace (as the config itself might contain confidential data). The name of the secret will be written into the resource's `.status` field:

```yaml
...
status:
  ...
  cloudConfig:
    secretRef:
      name: osc-result-pool-01-original
      namespace: default
  command: <machine configuration command>
  units:
  - docker-monitor.service
  - kubelet-monitor.service
  - kubelet.service
```
The secret has one data key `cloud_config` that stores the generation.

The generation of this operating system representation is executed by a [`Generator`](generator/generator.go). A default implementation for the `generator` based on [go templates](https://golang.org/pkg/text/template/) is provided in [`template`](template).

In addition, `oscommon` provides set of basic [`tests`](generator/test/README.md) which can be used to test the operating system specific generator.

Please find more information regarding the extensibility concepts and a detailed proposal [here](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md).

----

## How to use oscommon in a new operating system configuration controller

When implemening a controller for a specific operating system, it is necessary to provide:
* A command line application for launching the controller
* A template for translating the `cloud-config` to the format requried by the operating system.
* Alternatively, a new generator can also be provided, in case the transformations required by
the operating system requires more complex logic than provided by go templates.
* A test that uses the test description provided in [`pkg/generator/test`]
* A directory with test files
* The [`helm`](https://github.com/helm/helm) Chart for operator registration and installation

Please refer to the [`os-suse-jeos controller`](https://github.com/gardener/gardener-extension-os-suse-jeos) for a concrete example.

## Feedback and Support

Feedback and contributions are always welcome. Please report bugs or suggestions as [GitHub issues](https://github.com/gardener/gardener/issues) or join our [Slack channel #gardener](https://kubernetes.slack.com/messages/gardener) (please invite yourself to the Kubernetes workspace [here](http://slack.k8s.io)).

## Learn more!

Please find further resources about out project here:

* [Our landing page gardener.cloud](https://gardener.cloud/)
* ["Gardener, the Kubernetes Botanist" blog on kubernetes.io](https://kubernetes.io/blog/2018/05/17/gardener/)
* [GEP-1 (Gardener Enhancement Proposal) on extensibility](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md)
