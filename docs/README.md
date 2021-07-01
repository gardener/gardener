# Documentation Index

## Overview

* [General Architecture](concepts/architecture.md)
* [Gardener landing page `gardener.cloud`](https://gardener.cloud/)
* ["Gardener, the Kubernetes Botanist" blog on kubernetes.io](https://kubernetes.io/blog/2018/05/17/gardener/)
* ["Gardener Project Update" blog on kubernetes.io](https://kubernetes.io/blog/2019/12/02/gardener-project-update/)

## Concepts

* Components
  * [Gardener API server](concepts/apiserver.md)
    * [In-Tree admission plugins](concepts/apiserver_admission_plugins.md)
  * [Gardener Controller Manager](concepts/controller-manager.md)
  * [Gardener Scheduler](concepts/scheduler.md)
  * [Gardener Admission Controller](concepts/admission-controller.md)
  * [Gardener Seed Admission Controller](concepts/seed-admission-controller.md)
  * [Gardenlet](concepts/gardenlet.md)
* [Backup Restore](concepts/backup-restore.md)
* [Network Policies](concepts/network_policies.md)
* [Relation between Gardener API and Cluster API](concepts/cluster-api.md)

## Usage

* [APIServerSNI environment variable injection](usage/apiserver-sni-injection.md)
* [Audit a Kubernetes cluster](usage/shoot_auditpolicy.md)
* [Auto-Scaling for shoot clusters](usage/shoot_autoscaling.md)
* [Cleanup of Shoot clusters in deletion](usage/shoot_cleanup.md)
* [Custom `CoreDNS` configuration](usage/custom-dns.md)
* [(Custom) CSI components](usage/csi_components.md)
* [Gardener configuration and usage](usage/configuration.md)
* [`ManagedIstio` feature](usage/istio.md)
* [Network Policies in the Shoot Cluster](usage/shoot_network_policies.md)
* [`NodeLocalDNS` feature](usage/node-local-dns.md)
* [OpenIDConnect presets](usage/openidconnect-presets.md)
* [Projects](usage/projects.md)
* [Register Shoot as Seed](usage/managed_seed.md)
* [Reversed Cluster VPN](usage/reversed-vpn-tunnel.md)
* [Seed Bootstrapping](usage/seed_bootstrapping.md)
* [Seed Settings](usage/seed_settings.md)
* [Shoot cluster purposes](usage/shoot_purposes.md)
* [Shoot Kubernetes and Operating System Versioning](usage/shoot_versions.md)
* [Shoot Networking](usage/shoot_networking.md)
* [Shoot Maintenance](usage/shoot_maintenance.md)
* [Shoot Status](usage/shoot_status.md)
* [Shoot Updates and Upgrades](usage/shoot_updates.md)
* [Supported Kubernetes versions](usage/supported_k8s_versions.md)
* [Tolerations](usage/tolerations.md)
* [Trigger shoot operations](usage/shoot_operations.md)
* [Troubleshooting guide](usage/trouble_shooting_guide.md)
* [Trusted TLS certificate for shoot control planes](usage/trusted-tls-for-control-planes.md)

## Proposals

* [GEP: Gardener Enhancement Proposal Description](proposals/README.md)
* [GEP: Template](proposals/00-template.md)
* [GEP-1: Gardener extensibility and extraction of cloud-specific/OS-specific knowledge](proposals/01-extensibility.md)
* [GEP-2: `BackupInfrastructure` CRD and Controller Redesign](proposals/02-backupinfra.md)
* [GEP-3: Network extensibility](proposals/03-networking-extensibility.md)
* [GEP-4: New `core.gardener.cloud/v1alpha1` APIs required to extract cloud-specific/OS-specific knowledge out of Gardener core](proposals/04-new-core-gardener-cloud-apis.md)
* [GEP-5: Gardener Versioning Policy](proposals/05-versioning-policy.md)
* [GEP-6: Integrating etcd-druid with Gardener](proposals/06-etcd-druid.md)
* [GEP-7: Shoot Control Plane Migration](proposals/07-shoot-control-plane-migration.md)
* [GEP-8: SNI Passthrough proxy for kube-apiservers](proposals/08-shoot-apiserver-via-sni.md)
* [GEP-9: Gardener integration test framework](proposals/09-test-framework.md)
* [GEP-10: Support additional container runtimes](proposals/10-shoot-additional-container-runtimes.md)
* [GEP-11: Utilize API Server Network Proxy to Invert Seed-to-Shoot Connectivity](proposals/11-apiserver-network-proxy.md)
* [GEP-12: OIDC Webhook Authenticator](proposals/12-oidc-webhook-authenticator.md)
* [GEP-13: Automated Seed Management](proposals/13-automated-seed-management.md)
* [GEP-14: Reversed Cluster VPN](proposals/14-reversed-cluster-vpn.md)
* [GEP-15: Manage Bastions and SSH Key Pair Rotation](proposals/15-manage-bastions-and-ssh-key-pair-rotation.md)
* [GEP-16: Dynamic kubeconfig generation for Shoot clusters](proposals/16-adminkubeconfig-subresource.md)

## Development

* [Setting up a local development environment](development/local_setup.md)
* [Unit Testing and Dependency Management](development/testing_and_dependencies.md)
* [Changing the API](development/changing-the-api.md)
* [Releases, Features, Hotfixes](development/process.md)
* [Adding New Cloud Providers](development/new-cloud-provider.md)
* [Extending the Monitoring Stack](development/monitoring-stack.md)
* [How to create log parser for container into fluent-bit](development/log_parsers.md)
* [Network Policies in the Seed Cluster](development/seed_network_policies.md)

## Extensions

* [Extensibility overview](extensions/overview.md)
* [Extension controller registration](extensions/controllerregistration.md)
* [`Cluster` resource](extensions/cluster.md)
* Extension points
  * [General conventions](extensions/conventions.md)
  * [Trigger for reconcile operations](extensions/reconcile-trigger.md)
  * [Deploy resources into the shoot cluster](extensions/managedresources.md)
  * [Shoot resource customization webhooks](extensions/shoot-webhooks.md)
  * [Logging and monitoring for extensions](extensions/logging-and-monitoring.md)
  * [Contributing to shoot health status conditions](extensions/shoot-health-status-conditions.md)
    * [Health Check Library](extensions/healthcheck-library.md)
  * Blob storage providers
    * [`BackupBucket` resource](extensions/backupbucket.md)
    * [`BackupEntry` resource](extensions/backupentry.md)
  * DNS providers
    * [`DNSProvider` and `DNSEntry` resources](extensions/dns.md)
    * [`DNSRecord` resources](extensions/dnsrecord.md)
  * IaaS/Cloud providers
    * [Control plane customization webhooks](extensions/controlplane-webhooks.md)
    * [`Bastion` resource](extensions/bastion.md)
    * [`ControlPlane` resource](extensions/controlplane.md)
    * [`ControlPlane` exposure resource](extensions/controlplane-exposure.md)
    * [`Infrastructure` resource](extensions/infrastructure.md)
    * [`Worker` resource](extensions/worker.md)
  * Network plugin providers
    * [`Network` resource](extensions/network.md)
  * Operating systems
    * [`OperatingSystemConfig` resource](extensions/operatingsystemconfig.md)
  * Container runtimes
    * [`ContainerRuntime` resource](extensions/containerruntime.md)
  * Generic (non-essential) extensions
    * [`Extension` resource](extensions/extension.md)
* [Extending project roles](extensions/project-roles.md)
* [Referenced resources](extensions/referenced-resources.md)
* [Control plane migration](extensions/migration.md)

## Testing

* [Integration Testing Manual](testing/integration_tests.md)

## Deployment

* [Setup Gardener on a Kubernetes cluster](deployment/setup_gardener.md)
* [Deploying Gardenlets](deployment/deploy_gardenlet.md)
    * [Automatic Deployment of Gardenlets](deployment/deploy_gardenlet_automatically.md)
    * [Deploy a Gardenlet Manually](deployment/deploy_gardenlet_manually.md)
    * [Scoped API Access for Gardenlets](deployment/gardenlet_api_access.md)
* [Deploying the Gardener and a Seed into an AKS cluster](deployment/aks.md)
* [Overwrite image vector](deployment/image_vector.md)
* [Migration from Gardener `v0` to `v1`](deployment/migration_v0_to_v1.md)
* [Feature Gates in Gardener](deployment/feature_gates.md)

## Monitoring

* [Alerting](monitoring/alerting.md)
* [User Alerts](monitoring/user_alerts.md)
* [Operator Alerts](monitoring/operator_alerts.md)
