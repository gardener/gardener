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

## Extensions

* [Extensibility overview](extensions/overview.md)
* [Extension controller registration](extensions/controllerregistration.md)
* [`Cluster` resource](extensions/cluster.md)
* Extension points
  * [General conventions](extensions/conventions.md)
  * [Trigger for reconcile operations](extensions/reconcile-trigger.md)
  * [Deploy resources into the shoot cluster](extensions/managedresources.md)
  * [Shoot resource customization webhooks](extensions/shoot-webhooks.md)
  * DNS providers
    * [`DNSProvider` and `DNSEntry` resources](extensions/dns.md)
  * IaaS/Cloud providers
    * [Control plane customization webhooks](extensions/controlplane-webhooks.md)
    * [`ControlPlane` resource](extensions/controlplane.md)
    * [`Infrastructure` resource](extensions/infrastructure.md)
    * [`Worker` resource](extensions/worker.md)
  * Operating systems
    * [`OperatingSystemConfig` resource](extensions/operatingsystemconfig.md)
  * Other extensions
    * [`Extension` resource](extensions/extension.md)

## Deployment

* [Deploying the Gardener into a Kubernetes cluster](deployment/kubernetes.md)
* [Deploying the Gardener and a Seed into an AKS cluster](deployment/aks.md)
* [Overwrite image vector](deployment/image_vector.md)

## Usage

* [Audit a Kubernetes Cluster](usage/shoot_auditpolicy.md)
* [Supported Kubernetes versions](usage/supported_k8s_versions.md)
* [Trouble Shooting Guide](usage/trouble_shooting_guide.md)

## Proposals

* [Gardener extensibility and extraction of cloud-specific/OS-specific knowledge](proposals/01-extensibility.md)
