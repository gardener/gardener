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
  * [Gardener Resource Manager](concepts/resource-manager.md)
  * [Gardener Operator](concepts/operator.md)
  * [Gardener Node Agent](concepts/node-agent.md)
  * [Gardenlet](concepts/gardenlet.md)
* [Backup Restore](concepts/backup-restore.md)
* [etcd](concepts/etcd.md)
* [Relation between Gardener API and Cluster API](concepts/cluster-api.md)

## Usage

* [Audit a Kubernetes cluster](usage/shoot_auditpolicy.md)
* [Auto-Scaling for shoot clusters](usage/shoot_autoscaling.md)
* [Cleanup of Shoot clusters in deletion](usage/shoot_cleanup.md)
* [`containerd` Registry Configuration](usage/containerd-registry-configuration.md)
* [Custom `containerd` configuration](usage/custom-containerd-config.md)
* [Custom `CoreDNS` configuration](usage/custom-dns-config.md)
* [(Custom) CSI components](usage/csi_components.md)
* [Default Seccomp Profile](usage/default_seccomp_profile.md)
* [DNS Autoscaling](usage/dns-autoscaling.md)
* [DNS Search Path Optimization](usage/dns-search-path-optimization.md)
* [Endpoints and Ports of a Shoot Control-Plane](usage/control-plane-endpoints-and-ports.md)
* [ETCD Encryption Config](usage/etcd_encryption_config.md)
* [ExposureClasses](usage/exposureclasses.md)
* [Hibernate a Cluster](usage/shoot_hibernate.md)
* [IPv6 in Gardener Clusters](usage/ipv6.md)
* [Logging](usage/logging.md)
* [`NodeLocalDNS` feature](usage/node-local-dns.md)
* [OpenIDConnect presets](usage/openidconnect-presets.md)
* [Projects](usage/projects.md)
* [Service Account Manager](usage/service-account-manager.md)
* [Readiness of Shoot Worker Nodes](usage/node-readiness.md)
* [Reversed Cluster VPN](usage/reversed-vpn-tunnel.md)
* [Shoot Cluster Purposes](usage/shoot_purposes.md)
* [Shoot Scheduling Profiles](usage/shoot_scheduling_profiles.md)
* [Shoot Credentials Rotation](usage/shoot_credentials_rotation.md)
* [Shoot Kubernetes and Operating System Versioning](usage/shoot_versions.md)
* [Shoot `KUBERNETES_SERVICE_HOST` Environment Variable Injection](usage/shoot_kubernetes_service_host_injection.md)
* [Shoot Networking](usage/shoot_networking.md)
* [Shoot Maintenance](usage/shoot_maintenance.md)
* [Shoot `ServiceAccount` Configurations](usage/shoot_serviceaccounts.md)
* [Shoot Status](usage/shoot_status.md)
* [Shoot Info `ConfigMap`](usage/shoot_info_configmap.md)
* [Shoot Updates and Upgrades](usage/shoot_updates.md)
* [Shoot HA Control Plane](usage/shoot_high_availability.md)
* [Shoot HA Best Practices](usage/shoot_high_availability_best_practices.md)
* [Shoot Workers Settings](usage/shoot_workers_settings.md)
* [Accessing Shoot Clusters](usage/shoot_access.md)
* [Supported Kubernetes versions](usage/supported_k8s_versions.md)
* [Tolerations](usage/tolerations.md)
* [Trigger shoot operations](usage/shoot_operations.md)
* [Trusted TLS certificate for shoot control planes](usage/trusted-tls-for-control-planes.md)
* [Trusted TLS certificate for garden runtime cluster](usage/trusted-tls-for-garden-runtime.md)
* [Controlling the Kubernetes versions for specific worker pools](usage/worker_pool_k8s_versions.md)
* [Admission Configuration for the `PodSecurity` Admission Plugin](usage/pod-security.md)
* [Supported CPU Architectures for Shoot Worker Nodes](usage/shoot_supported_architectures.md)
* [Workerless `Shoot`s](usage/shoot_workerless.md)

## [API Reference](api-reference/README.md)

* [`authentication.gardener.cloud` API Group](api-reference/authentication.md)
* [`core.gardener.cloud` API Group](api-reference/core.md)
* [`extensions.gardener.cloud` API Group](api-reference/extensions.md)
* [`operations.gardener.cloud` API Group](api-reference/operations.md)
* [`resources.gardener.cloud` API Group](api-reference/resources.md)
* [`seedmanagement.gardener.cloud` API Group](api-reference/seedmanagement.md)
* [`settings.gardener.cloud` API Group](api-reference/settings.md)

## Proposals

* [GEP: Gardener Enhancement Proposal Description](proposals/README.md)
* [GEP: Template](proposals/00-template.md)
* [GEP-1: Gardener extensibility and extraction of cloud-specific/OS-specific knowledge](proposals/01-extensibility.md)
* [GEP-2: `BackupInfrastructure` CRD and Controller Redesign](proposals/02-backupinfra.md)
* [GEP-3: Network extensibility](proposals/03-networking-extensibility.md)
* [GEP-4: New `core.gardener.cloud/v1beta1` APIs required to extract cloud-specific/OS-specific knowledge out of Gardener core](proposals/04-new-core-gardener-cloud-apis.md)
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
* [GEP-17: Shoot Control Plane Migration "Bad Case" Scenario](proposals/17-shoot-control-plane-migration-bad-case.md)
* [GEP-18: Automated Shoot CA Rotation](proposals/18-shoot-CA-rotation.md)
* [GEP-19: Observability Stack - Migrating to the prometheus-operator and fluent-bit operator](proposals/19-migrating-observability-stack-to-operators.md)
* [GEP-20: Highly Available Shoot Control Planes](proposals/20-ha-control-planes.md)
* [GEP-21: IPv6 Single-Stack Support in Local Gardener](proposals/21-ipv6-singlestack-local.md)
* [GEP-22: Improved Usage of the `ShootState` API](proposals/22-improved-usage-of-shootstate-api.md)
* [GEP-23: Autoscaling Shoot kube-apiserver via Independently Driven HPA and VPA](proposals/23-autoscaling-kube-apiserver-via-independent-hpa-and-vpa.md)
* [GEP-24: Shoot OIDC Issuer](proposals/24-shoot-oidc-issuer.md)
* [GEP-25: Namespaced Cloud Profiles](proposals/25-namespaced-cloud-profiles.md)
* [GEP-26: Workload Identity - Trust Based Authentication](proposals/26-workload-identity.md)

## Development

* [Getting started locally (using the local provider)](development/getting_started_locally.md)
* [Setting up a development environment (using a cloud provider)](development/local_setup.md)
* [Testing (Unit, Integration, E2E Tests)](development/testing.md)
* [Test Machinery Tests](development/testmachinery_tests.md)
* [Dependency Management](development/dependencies.md)
* [Kubernetes Clients in Gardener](development/kubernetes-clients.md)
* [Logging in Gardener Components](development/logging.md)
* [Changing the API](development/changing-the-api.md)
* [Secrets Management for Seed and Shoot Clusters](development/secrets_management.md)
* [Releases, Features, Hotfixes](development/process.md)
* [Adding New Cloud Providers](development/new-cloud-provider.md)
* [Adding Support For A New Kubernetes Version](development/new-kubernetes-version.md)
* [Extending the Monitoring Stack](development/monitoring-stack.md)
* [How to create log parser for container into fluent-bit](development/log_parsers.md)
* [`PriorityClasses` in Gardener Clusters](development/priority-classes.md)
* [High Availability Of Deployed Components](development/high-availability.md)
* [Checklist For Adding New Components](development/component-checklist.md)
* [Defaulting Strategy and Developer Guideline](development/defaulting.md)

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
  * [CA Rotation in Extensions](extensions/ca-rotation.md)
  * Blob storage providers
    * [`BackupBucket` resource](extensions/backupbucket.md)
    * [`BackupEntry` resource](extensions/backupentry.md)
  * DNS providers
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
  * [Extension Admission](extensions/admission.md)
  * [Heartbeat controller](extensions/heartbeat.md)
* [Provider Local](extensions/provider-local.md)
* [Access to the Garden Cluster](extensions/garden-api-access.md)
* [Control plane migration](extensions/migration.md)
* [Force Deletion](extensions/force-deletion.md)
* [Extending project roles](extensions/project-roles.md)
* [Referenced resources](extensions/referenced-resources.md)

## Deployment

* [Getting started locally](deployment/getting_started_locally.md)
* [Getting started locally with extensions](deployment/getting_started_locally_with_extensions.md)
* [Setup Gardener on a Kubernetes cluster](deployment/setup_gardener.md)
* [Version Skew Policy](deployment/version_skew_policy.md)
* [Deploying Gardenlets](deployment/deploy_gardenlet.md)
  * [Automatic Deployment of Gardenlets](deployment/deploy_gardenlet_automatically.md)
  * [Deploy a Gardenlet Manually](deployment/deploy_gardenlet_manually.md)
  * [Scoped API Access for Gardenlets](deployment/gardenlet_api_access.md)
* [Overwrite image vector](deployment/image_vector.md)
* [Migration from Gardener `v0` to `v1`](deployment/migration_v0_to_v1.md)
* [Feature Gates in Gardener](deployment/feature_gates.md)
* [Configuring the Logging stack](deployment/configuring_logging.md)
* [SecretBinding Provider Controller](deployment/secret_binding_provider_controller.md)

## Operations

* [Gardener configuration and usage](operations/configuration.md)
* [Control Plane Migration](operations/control_plane_migration.md)
* [Istio](operations/istio.md)
* [Register Shoot as Seed](operations/managed_seed.md)
* [`NetworkPolicy`s In Garden, Seed, Shoot Clusters](operations/network_policies.md)
* [Seed Bootstrapping](operations/seed_bootstrapping.md)
* [Seed Settings](operations/seed_settings.md)
* [Topology-Aware Traffic Routing](operations/topology_aware_routing.md)

## Monitoring

* [Alerting](monitoring/alerting.md)
* [Connectivity](monitoring/connectivity.md)
* [Operator Alerts](monitoring/operator_alerts.md)
* [Profiling Gardener Components](monitoring/profiling.md)
* [User Alerts](monitoring/user_alerts.md)