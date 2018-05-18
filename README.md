# [Gardener](https://gardener.cloud)

![Gardener Logo](logo/gardener-large.png)

[![Go Report Card](https://goreportcard.com/badge/github.com/gardener/gardener)](https://goreportcard.com/report/github.com/gardener/gardener)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1822/badge)](https://bestpractices.coreinfrastructure.org/projects/1822)

The Gardener implements the automated management and operation of [Kubernetes](https://kubernetes.io/) clusters as a service and aims to support that service on multiple Cloud providers (AWS, GCP, Azure, OpenStack). Its main principle is to use Kubernetes itself as base for its tasks.

In essence, the Gardener is an [extension API server](https://kubernetes.io/docs/tasks/access-kubernetes-api/setup-extension-api-server/) along with a bundle of Kubernetes controllers which introduces new API objects in an existing Kubernetes cluster (which is called **Garden** cluster) in order to use them for the management of further Kubernetes clusters (which are called **Shoot** clusters).
To do that reliably and to offer a certain quality of service, it requires to control the main components of a Kubernetes cluster (etcd, API server, controller manager, scheduler). These so-called *control plane* components are hosted in Kubernetes clusters themselves (which are called **Seed** clusters).

Please find more information regarding the concepts and a detailed description of the architecture in our [Gardener Wiki](https://github.com/gardener/documentation/wiki/Architecture) and our [blog post on kubernetes.io](https://kubernetes.io/blog/2018/05/17/gardener/).

----

## To start using or developing the Gardener locally

See our documentation in the `/docs` repository, please [find the index here](docs/README.md).

## Setting up your own Gardener landscape in the cloud

Take a look at our [landscape setup template](https://github.com/gardener/landscape-setup-template) to bootstrap your own Gardener system.

## Feedback and Support

Feedback and contributions are always welcome. Please report bugs or suggestions about our Kubernetes clusters as such or the Gardener itself as [GitHub issues](https://github.com/gardener/gardener/issues) or join our [Slack workspace](https://gardener-cloud.slack.com) (find the self-service invitation link [here](https://publicslack.com/slacks/gardener-cloud/invites/new)).
