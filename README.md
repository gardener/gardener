# [Gardener](https://gardener.cloud)

![Gardener Logo](logo/gardener-large.png)

[![Slack channel #gardener](https://img.shields.io/badge/slack-gardener-brightgreen.svg?logo=slack)](https://kubernetes.slack.com/messages/gardener)
[![Go Report Card](https://goreportcard.com/badge/github.com/gardener/gardener)](https://goreportcard.com/report/github.com/gardener/gardener)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1822/badge)](https://bestpractices.coreinfrastructure.org/projects/1822)

Gardener implements the automated management and operation of [Kubernetes](https://kubernetes.io/) clusters as a service and provides support for multiple cloud providers (Alicloud, AWS, Azure, GCP, OpenStack, ...). Its main principle is to leverage Kubernetes concepts for all of its tasks.

In essence, Gardener is an [extension API server](https://kubernetes.io/docs/tasks/access-kubernetes-api/setup-extension-api-server/) that comes along with a bundle of custom controllers. It introduces new API objects in an existing Kubernetes cluster (which is called **garden** cluster) in order to use them for the management of end-user Kubernetes clusters (which are called **shoot** clusters). These shoot clusters are described via [declarative cluster specifications](https://github.com/gardener/gardener/blob/master/example/90-shoot-aws.yaml) which are observed by the controllers. They will bring up the clusters, reconcile their state, perform automated updates and make sure they are always up and running.

To accomplish these tasks reliably and to offer a certain quality of service, Gardener requires to control the main components of a Kubernetes cluster (etcd, API server, controller manager, scheduler). These so-called *control plane* components are hosted in Kubernetes clusters themselves (which are called **seed** clusters). This is the main difference compared to many other OSS cluster provisioning tools: The shoot clusters do not have dedicated master VMs, instead, the control plane is deployed as native Kubernetes workload into the seeds. This does not only effectively reducing the total costs of ownership, it also allows easier implementations for "day-2 operations" (like cluster updates or robustness) by relying on all the mature Kubernetes features and capabilities.

Please find more information regarding the concepts and a detailed description of the architecture in our [Gardener Wiki](https://github.com/gardener/documentation/wiki/Architecture) and our [blog post on kubernetes.io](https://kubernetes.io/blog/2018/05/17/gardener/).

----

## To start using or developing the Gardener locally

See our documentation in the `/docs` repository, please [find the index here](docs/README.md).

## Setting up your own Gardener landscape in the cloud

The quickest way to test drive Gardener is to install it virtually onto an existing Kubernetes cluster, just like you would install any other Kubernetes-ready application. Launch your automatic installer [here](https://gardener.cloud/installer/)

We also have a [Gardener Helm Chart](https://github.com/gardener/gardener/tree/master/charts/gardener). Alternatively you can use our [garden setup](https://github.com/gardener/garden-setup) project to create a fully configured Gardener landscape which also includes our [Gardener Dashboard](https://github.com/gardener/dashboard).

## Feedback and Support

Feedback and contributions are always welcome!

All channels for getting in touch or learning about our project are listed under the [community](https://github.com/gardener/documentation/blob/master/CONTRIBUTING.md#community) section. We are cordially inviting interested parties to join our [weekly meetings](https://github.com/gardener/documentation/blob/master/CONTRIBUTING.md#weekly-meeting).

Please report bugs or suggestions about our Kubernetes clusters as such or the Gardener itself as [GitHub issues](https://github.com/gardener/gardener/issues) or join our [Slack channel #gardener](https://kubernetes.slack.com/messages/gardener) (please invite yourself to the Kubernetes workspace [here](http://slack.k8s.io)).

## Learn more!

Please find further resources about out project here:

* [Our landing page gardener.cloud](https://gardener.cloud/)
* ["Gardener, the Kubernetes Botanist" blog on kubernetes.io](https://kubernetes.io/blog/2018/05/17/gardener/)
* [SAP news article about "Project Gardener"](https://news.sap.com/2018/11/hasso-plattner-founders-award-finalist-profile-project-gardener/)
* [Introduction movie: "Gardener - Planting the Seeds of Success in the Cloud"](https://www.sap-tv.com/m/video/40962/gardener-planting-the-seeds-of-success-in-the-cloud)
* ["Thinking Cloud Native" talk at EclipseCon 2018](https://www.youtube.com/watch?v=bfw22WPg99A)
* [Blog - "Showcase of Gardener at OSCON 2018"](https://blogs.sap.com/2018/07/26/showcase-of-gardener-at-oscon/)
