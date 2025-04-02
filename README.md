# [Gardener](https://gardener.cloud)

![Gardener Logo](logo/gardener-large.png)

[![REUSE status](https://api.reuse.software/badge/github.com/gardener/gardener)](https://api.reuse.software/info/github.com/gardener/gardener)
[![CI Build status](https://concourse.ci.gardener.cloud/api/v1/teams/gardener/pipelines/gardener-master/jobs/master-head-update-job/badge)](https://concourse.ci.gardener.cloud/teams/gardener/pipelines/gardener-master/jobs/master-head-update-job)
[![Slack channel #gardener](https://img.shields.io/badge/slack-gardener-brightgreen.svg?logo=slack)](https://kubernetes.slack.com/messages/gardener)
[![Go Report Card](https://goreportcard.com/badge/github.com/gardener/gardener)](https://goreportcard.com/report/github.com/gardener/gardener)
[![GoDoc](https://godoc.org/github.com/gardener/gardener?status.svg)](https://godoc.org/github.com/gardener/gardener)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1822/badge)](https://bestpractices.coreinfrastructure.org/projects/1822)

Gardener implements the automated management and operation of [Kubernetes](https://kubernetes.io/) clusters as a service and provides a fully validated extensibility framework that can be adjusted to any programmatic cloud or infrastructure provider.

Gardener is 100% Kubernetes-native and exposes its own Cluster API to create homogeneous clusters on all supported infrastructures. This API differs from [SIG Cluster Lifecycle](https://github.com/kubernetes/community/tree/master/sig-cluster-lifecycle)'s [Cluster API](https://github.com/kubernetes-sigs/cluster-api#cluster-api) that only harmonizes how to get to clusters, while [Gardener's Cluster API](./docs/api-reference/core.md#shoot) goes one step further and also harmonizes the make-up of the clusters themselves. That means, Gardener gives you homogeneous clusters with exactly the same bill of material, configuration and behavior on all supported infrastructures, which you can see further down below in the section on our K8s Conformance Test Coverage.

In 2020, SIG Cluster Lifecycle's Cluster API made a huge step forward with [`v1alpha3`](https://kubernetes.io/blog/2020/04/21/cluster-api-v1alpha3-delivers-new-features-and-an-improved-user-experience/) and the newly added support for declarative control plane management. This made it possible to integrate managed services like GKE or Gardener. We would be more than happy, if the community would be interested, to contribute a Gardener control plane provider. For more information on the relation between Gardener API and SIG Cluster Lifecycle's Cluster API, please see [here](docs/concepts/cluster-api.md).

Gardener's main principle is to **leverage Kubernetes concepts for all of its tasks**.

In essence, Gardener is an [extension API server](https://kubernetes.io/docs/tasks/access-kubernetes-api/setup-extension-api-server/) that comes along with a bundle of custom controllers. It introduces new API objects in an existing Kubernetes cluster (which is called **garden** cluster) in order to use them for the management of end-user Kubernetes clusters (which are called **shoot** clusters). These shoot clusters are described via [declarative cluster specifications](https://github.com/gardener/gardener/blob/master/example/90-shoot.yaml) which are observed by the controllers. They will bring up the clusters, reconcile their state, perform automated updates and make sure they are always up and running.

To accomplish these tasks reliably and to offer a high quality of service, Gardener controls the main components of a Kubernetes cluster (etcd, API server, controller manager, scheduler). These so-called *control plane* components are hosted in Kubernetes clusters themselves (which are called **seed** clusters). This is the main difference compared to many other OSS cluster provisioning tools: The shoot clusters do not have dedicated master VMs. Instead, the control plane is deployed as a native Kubernetes workload into the seeds (the architecture is commonly referred to as kubeception or inception design). This does not only effectively reduce the total cost of ownership but also allows easier implementations for "day-2 operations" (like cluster updates or robustness) by relying on all the mature Kubernetes features and capabilities.

Gardener reuses the identical Kubernetes design to span a scalable multi-cloud and multi-cluster landscape. Such familiarity with known concepts has proven to quickly ease the initial learning curve and accelerate developer productivity:

* Kubernetes API Server = Gardener API Server
* Kubernetes Controller Manager = Gardener Controller Manager
* Kubernetes Scheduler = Gardener Scheduler
* Kubelet = Gardenlet
* Node = Seed cluster
* Pod = Shoot cluster

Please find more information regarding the concepts and a detailed description of the architecture in our [Gardener Wiki](https://github.com/gardener/gardener/blob/master/docs/concepts/architecture.md) and our blog posts on kubernetes.io: [Gardener - the Kubernetes Botanist (17.5.2018)](https://kubernetes.io/blog/2018/05/17/gardener) and [Gardener Project Update (2.12.2019)](https://kubernetes.io/blog/2019/12/02/gardener-project-update).

----

## K8s Conformance Test Coverage <img src="https://raw.githubusercontent.com/cncf/artwork/main/projects/kubernetes/certified-kubernetes/versionless/color/certified-kubernetes-color.svg" alt="certified kubernetes logo" width="50" align="right"/>

Gardener takes part in the [Certified Kubernetes Conformance Program](https://www.cncf.io/certification/software-conformance/) to attest its compatibility with the K8s conformance testsuite.
Currently, Gardener is certified for K8s versions up to v1.31, see [the conformance spreadsheet](https://docs.google.com/spreadsheets/d/1uF9BoDzzisHSQemXHIKegMhuythuq_GL3N1mlUUK2h0/edit#gid=0&range=107:108).

Continuous conformance test results of the latest stable Gardener release are uploaded regularly to the CNCF test grid:

| Provider/K8s | v1.32 | v1.31 | v1.30 | v1.29 | v1.28 | v1.27 |
| ------------ |-----| ------------ | ------------ | ------------ | ------------ | ------------ |
| **AWS** | [![Gardener v1.32 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.32%20AWS/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.32%20AWS) | [![Gardener v1.31 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.31%20AWS/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.31%20AWS) | [![Gardener v1.30 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.30%20AWS/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.30%20AWS) | [![Gardener v1.29 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.29%20AWS/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.29%20AWS) | [![Gardener v1.28 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.28%20AWS/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.28%20AWS) | [![Gardener v1.27 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.27%20AWS/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.27%20AWS) |
| **Azure** | [![Gardener v1.32 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.32%20Azure/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.32%20Azure) | [![Gardener v1.31 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.31%20Azure/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.31%20Azure) | [![Gardener v1.30 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.30%20Azure/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.30%20Azure) | [![Gardener v1.29 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.29%20Azure/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.29%20Azure) | [![Gardener v1.28 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.28%20Azure/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.28%20Azure) | [![Gardener v1.27 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.27%20Azure/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.27%20Azure) |
| **GCP** | [![Gardener v1.32 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.32%20GCE/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.32%20GCE) | [![Gardener v1.31 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.31%20GCE/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.31%20GCE) | [![Gardener v1.30 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.30%20GCE/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.30%20GCE) | [![Gardener v1.29 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.29%20GCE/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.29%20GCE) | [![Gardener v1.28 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.28%20GCE/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.28%20GCE) | [![Gardener v1.27 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.27%20GCE/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.27%20GCE) |
| **OpenStack** | [![Gardener v1.32 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.32%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.32%20OpenStack) | [![Gardener v1.31 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.31%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.31%20OpenStack) | [![Gardener v1.30 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.30%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.30%20OpenStack) | [![Gardener v1.29 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.29%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.29%20OpenStack) | [![Gardener v1.28 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.28%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.28%20OpenStack) | [![Gardener v1.27 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.27%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.27%20OpenStack) |
| **Alicloud** | [![Gardener v1.32 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.32%20Alibaba%20Cloud/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.32%20Alibaba%20Cloud) | [![Gardener v1.31 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.31%20Alibaba%20Cloud/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.31%20Alibaba%20Cloud) | [![Gardener v1.30 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.30%20Alibaba%20Cloud/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.30%20Alibaba%20Cloud) | [![Gardener v1.29 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.29%20Alibaba%20Cloud/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.29%20Alibaba%20Cloud) | [![Gardener v1.28 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.28%20Alibaba%20Cloud/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.28%20Alibaba%20Cloud) | [![Gardener v1.27 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.27%20Alibaba%20Cloud/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.27%20Alibaba%20Cloud) |

Get an overview of the test results at [testgrid](https://testgrid.k8s.io/conformance-gardener).

## Quickstart with the demo environment

Check out our [Gardener demo environment](https://demo.gardener.cloud)!

It is a preconfigured playground which includes lots of tutorials and examples to get you started with Gardener.

## Start using or developing the Gardener locally

See our documentation in the `/docs` repository, please [find the index here](docs/README.md).

## Setting up your own Gardener landscape

Bootstrapping and maintaining a Gardener landscape has never been easier. Check out our [Gardener landscape setup guide](docs/deployment/setup_gardener.md) to learn about the operator and other key concepts.

## Feedback and Support

Feedback and contributions are always welcome!

All channels for getting in touch or learning about our project are listed under the [community](https://gardener.cloud/docs/contribute/#community) section. We are cordially inviting interested parties to join our [bi-weekly meetings](https://gardener.cloud/docs/contribute/#bi-weekly-meetings).

Please report bugs or suggestions about our Kubernetes clusters as such or the Gardener itself as [GitHub issues](https://github.com/gardener/gardener/issues) or join our [Slack channel #gardener](https://kubernetes.slack.com/messages/gardener) (please invite yourself to the Kubernetes workspace [here](http://slack.k8s.io)).

## Learn More!

Please find further resources about our project here:

* [Our landing page gardener.cloud](https://gardener.cloud/)
* ["Gardener Project Update" blog on kubernetes.io](https://kubernetes.io/blog/2019/12/02/gardener-project-update/).
* ["Gardener, the Kubernetes Botanist" blog on kubernetes.io](https://kubernetes.io/blog/2018/05/17/gardener/)
* ["Thinking Cloud Native" talk at EclipseCon 2018](https://www.youtube.com/watch?v=bfw22WPg99A)
* [Blog - "Showcase of Gardener at OSCON 2018"](https://blogs.sap.com/2018/07/26/showcase-of-gardener-at-oscon/)
