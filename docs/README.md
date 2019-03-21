# Documentation Index

## Development

* [Setting up a local development environment](development/local_setup.md)
* [Unit Testing and Dependency Management](development/testing_and_dependencies.md)
* [Integration Testing](testing/integration_tests.md)
* [Features, Releases and Hotfixes](development/process.md)
* [Adding New Cloud Providers](development/new-cloud-provider.md)
* [Extending the Monitoring Stack](development/monitoring-stack.md)

## Concepts

* [Configuration and Secrets](concepts/configuration.md)
* [Machine-Controller-Manager integration and Machine bootstrap flow](concepts/machine-bootstrap.md)
* [Repositories of required components](concepts/repositories.md)

## Extensions

* [Extensibility overview](extensions/overview.md)
* [Extension controller registration](extensions/controllerregistration.md)
* Contracts
  * [`DNSProvider` and `DNSEntry` resources](extensions/dns.md)
  * [`OperatingSystemConfig` resource](extensions/operatingsystemconfig.md)

## Deployment

* [Deploying the Gardener into a Kubernetes cluster](deployment/kubernetes.md)
* [Deploying the Gardener into a Minikube with Local Provider](deployment/kubernetes-minikube.md)
* [Deploying the Gardener and a Seed into an AKS cluster](deployment/aks.md)
* [Overwrite image vector](deployment/image_vector.md)

## Usage

* [Creating, deleting and updating Shoot clusters](usage/shoots.md)
* [Audit a Kubernetes Cluster](usage/shoot_auditpolicy.md)
* [Supported Kubernetes versions](usage/supported_k8s_versions.md)

## Proposals

* [Gardener extensibility and extraction of cloud-specific/OS-specific knowledge](proposals/01-extensibility.md)
