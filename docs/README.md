# Documentation Index

## Overview

* [General Architecture](concepts/architecture.md)
* [Gardener landing page `gardener.cloud`](https://gardener.cloud/)
* ["Gardener, the Kubernetes Botanist" blog on kubernetes.io](https://kubernetes.io/blog/2018/05/17/gardener/)
* ["Gardener Project Update" blog on kubernetes.io](https://kubernetes.io/blog/2019/12/02/gardener-project-update/)

## Concepts

* Components
  * [Gardener API server](concepts/apiserver.md)
    * [In-Tree Admission Plugins](concepts/apiserver-admission-plugins.md)
  * [Gardener Controller Manager](concepts/controller-manager.md)
  * [Gardener Scheduler](concepts/scheduler.md)
  * [Gardener Admission Controller](concepts/admission-controller.md)
  * [Gardener Resource Manager](concepts/resource-manager.md)
  * [Gardener Operator](concepts/operator.md)
  * [Gardener Node Agent](concepts/node-agent.md)
  * [Gardenlet](concepts/gardenlet.md)
  * [Gardenadm](concepts/gardenadm.md)
* [Backup Restore](concepts/backup-restore.md)
* [etcd](concepts/etcd.md)
* [Relation between Gardener API and Cluster API](concepts/cluster-api.md)

## Usage

### Gardener

* [Gardener Info `ConfigMap`](operations/configmap.md)

### Project

* [Projects](usage/project/projects.md)
* [Service Account Manager](usage/project/service-account-manager.md)
* [`NamespacedCloudProfile`s](usage/project/namespaced-cloud-profiles.md)

### Shoot

* [Accessing Shoot Clusters](usage/shoot/shoot_access.md)
* [Hibernate a Cluster](usage/shoot/shoot_hibernate.md)
* [Shoot Info `ConfigMap`](usage/shoot/shoot_info_configmap.md)
* [Shoot Kubernetes Minor Version Upgrades](usage/shoot/shoot_kubernetes_versions.md)
* [Shoot Cluster Limits](usage/shoot/shoot_limits.md)
* [Shoot Maintenance](usage/shoot/shoot_maintenance.md)
* [Shoot Cluster Purposes](usage/shoot/shoot_purposes.md)
* [Shoot Scheduling Profiles](usage/shoot/shoot_scheduling_profiles.md)
* [Shoot Status](usage/shoot/shoot_status.md)
* [Supported CPU Architectures for Shoot Worker Nodes](usage/shoot/shoot_supported_architectures.md)
* [Workerless `Shoot`s](usage/shoot/shoot_workerless.md)
* [Shoot Workers Settings](usage/shoot/shoot_workers_settings.md)
* [Access Restrictions](usage/shoot/access_restrictions.md)
* [Workload Identity](usage/shoot/shoot-workload-identity.md)

### Shoot Operations

* [Shoot Credentials Rotation](usage/shoot-operations/shoot_credentials_rotation.md)
* [Trigger shoot operations](usage/shoot-operations/shoot_operations.md)
* [Shoot Updates and Upgrades](usage/shoot-operations/shoot_updates.md)
* [Shoot Kubernetes and Operating System Versioning](usage/shoot-operations/shoot_versions.md)
* [Supported Kubernetes versions](usage/shoot-operations/supported_k8s_versions.md)
* [Controlling the Kubernetes versions for specific worker pools](usage/shoot-operations/worker_pool_k8s_versions.md)
* [Migration from SecretBinding to CredentialsBinding](usage/shoot-operations/secretbinding-to-credentialsbinding-migration.md)
* [Manual Worker Pool Rollout](usage/shoot-operations/worker_pool_manual_rollout.md)

### High Availability

* [Shoot High-Availability Control Plane](usage/high-availability/shoot_high_availability.md)
* [Shoot High-Availability Best Practices](usage/high-availability/shoot_high_availability_best_practices.md)

### Security

* [Default Seccomp Profile](usage/security/default_seccomp_profile.md)
* [ETCD Encryption Config](usage/security/etcd_encryption_config.md)
* [OpenIDConnect presets](usage/security/openidconnect-presets.md)
* [Admission Configuration for the `PodSecurity` Admission Plugin](usage/security/pod-security.md)
* [Audit a Kubernetes cluster](usage/security/shoot_auditpolicy.md)
* [Shoot `ServiceAccount` Configurations](usage/security/shoot_serviceaccounts.md)

### Networking

* [Custom `CoreDNS` configuration](usage/networking/custom-dns-config.md)
* [DNS Search Path Optimization](usage/networking/dns-search-path-optimization.md)
* [ExposureClasses](usage/networking/exposureclasses.md)
* [`NodeLocalDNS` feature](usage/networking/node-local-dns.md)
* [Shoot `KUBERNETES_SERVICE_HOST` Environment Variable Injection](usage/networking/shoot_kubernetes_service_host_injection.md)
* [Shoot Networking](usage/networking/shoot_networking.md)
* [Dual-Stack Network Migration](usage/networking/dual-stack-networking-migration.md)

### Autoscaling

* [DNS Autoscaling](usage/autoscaling/dns-autoscaling.md)
* [In-place Resource Updates](usage/autoscaling/in-place-resource-updates.md)
* [Shoot Auto-Scaling Configuration](usage/autoscaling/shoot_autoscaling.md)
* [Shoot Pod Auto-Scaling Best Practices](usage/autoscaling/shoot_pod_autoscaling_best_practices.md)

### Observability

* [Logging](usage/logging.md)

### Advanced

* [`containerd` Registry Configuration](usage/advanced/containerd-registry-configuration.md)
* [Endpoints and Ports of a Shoot Control-Plane](usage/advanced/control-plane-endpoints-and-ports.md)
* [(Custom) CSI components](usage/advanced/csi_components.md)
* [Custom `containerd` configuration](usage/advanced/custom-containerd-config.md)
* [Readiness of Shoot Worker Nodes](usage/advanced/node-readiness.md)
* [Cleanup of Shoot clusters in deletion](usage/advanced/shoot_cleanup.md)
* [Tolerations](usage/advanced/tolerations.md)

### Reference

* [Well-known labels and annotations](operations/well-known-labels-annotations.md) 

## [API Reference](api-reference/README.md)

* [`authentication.gardener.cloud` API Group](api-reference/authentication.md)
* [`core.gardener.cloud` API Group](api-reference/core.md)
* [`extensions.gardener.cloud` API Group](api-reference/extensions.md)
* [`operations.gardener.cloud` API Group](api-reference/operations.md)
* [`resources.gardener.cloud` API Group](api-reference/resources.md)
* [`security.gardener.cloud` API Group](api-reference/security.md)
* [`seedmanagement.gardener.cloud` API Group](api-reference/seedmanagement.md)
* [`settings.gardener.cloud` API Group](api-reference/settings.md)

## [CLI Reference](cli-reference/README.md)

* [`gardenadm`](cli-reference/gardenadm/gardenadm.md)

## Development

* [Getting started locally (using the local provider)](development/getting_started_locally.md)
* [Setting up a development environment (using a cloud provider)](development/local_setup.md)
* [Testing (Unit, Integration, E2E Tests)](development/testing.md)
* [Test Machinery Tests](development/testmachinery_tests.md)
* [Dependency Management](development/dependencies.md)
* [Kubernetes Clients in Gardener](development/kubernetes-clients.md)
* [Validation Guidelines](development/validation-guidelines.md)
* [Logging Guidelines in Gardener Components](development/logging-guidelines.md)
* [Changing the API](development/changing-the-api.md)
* [Secrets Management for Seed and Shoot Clusters](development/secrets_management.md)
* [IPv6 in Gardener Clusters](development/ipv6.md)
* [Releases, Features, Hotfixes](development/process.md)
* [Reversed Cluster VPN](development/reversed-vpn-tunnel.md)
* [Adding New Cloud Providers](development/new-cloud-provider.md)
* [Adding Support For A New Kubernetes Version](development/new-kubernetes-version.md)
* [Removing Support For a Kubernetes Version](development/remove-support-for-kubernetes-version.md)
* [Extending the Monitoring Stack](development/monitoring-stack.md)
* [Logging Stack](development/logging-stack.md)
* [How to create log parser for container into fluent-bit](development/log_parsers.md)
* [`PriorityClasses` in Gardener Clusters](development/priority-classes.md)
* [High Availability Of Deployed Components](development/high-availability-of-components.md)
* [Checklist For Adding New Components](development/component-checklist.md)
* [Defaulting Strategy and Developer Guideline](development/defaulting.md)
* [Autoscaling Specifics for Components](development/autoscaling-specifics-for-components.md)
* [Shoot Advertised Addresses](development/shoot-advertised-addresses.md)

## Extensions

* [Extensibility overview](extensions/overview.md)
* [Extension registration](extensions/registration.md)
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
    * [`BackupBucket` resource](extensions/resources/backupbucket.md)
    * [`BackupEntry` resource](extensions/resources/backupentry.md)
  * DNS providers
    * [`DNSRecord` resources](extensions/resources/dnsrecord.md)
  * IaaS/Cloud providers
    * [Control plane customization webhooks](extensions/controlplane-webhooks.md)
    * [`Bastion` resource](extensions/resources/bastion.md)
    * [`ControlPlane` resource](extensions/resources/controlplane.md)
    * [`Infrastructure` resource](extensions/resources/infrastructure.md)
    * [`SelfHostedShootExposure` resource](extensions/resources/selfhostedshootexposure.md)
    * [`Worker` resource](extensions/resources/worker.md)
  * Network plugin providers
    * [`Network` resource](extensions/resources/network.md)
  * Operating systems
    * [`OperatingSystemConfig` resource](extensions/resources/operatingsystemconfig.md)
  * Container runtimes
    * [`ContainerRuntime` resource](extensions/resources/containerruntime.md)
  * Generic (non-essential) extensions
    * [`Extension` resource](extensions/resources/extension.md)
  * [Extension Admission](extensions/admission.md)
  * [Heartbeat controller](extensions/heartbeat.md)
* [Provider Local](extensions/provider-local.md)
* [Access to the Garden Cluster](extensions/garden-api-access.md)
* [Control plane migration](extensions/migration.md)
* [Force Deletion](extensions/force-deletion.md)
* [Extending project roles](extensions/project-roles.md)
* [Referenced resources](extensions/referenced-resources.md)
* [Validation Guidelines For Extensions](extensions/validation-guidelines-for-extensions.md)
* [Static Manifest Propagation From Seed To Shoot](extensions/static-manifests.md)

## Deployment

* [Getting started locally](deployment/getting_started_locally.md)
* [Getting started locally with extensions](deployment/getting_started_locally_with_extensions.md)
* [Getting started locally with Self-Hosted Shoot Clusters](deployment/getting_started_locally_with_gardenadm.md)
* [Setup Gardener on a Kubernetes cluster](deployment/setup_gardener.md)
* [Version Skew Policy](deployment/version_skew_policy.md)
* [Deploying Gardenlets](deployment/deploy_gardenlet.md)
  * [Automatic Deployment of Gardenlets](deployment/deploy_gardenlet_automatically.md)
  * [Deploy a Gardenlet Manually](deployment/deploy_gardenlet_manually.md)
  * [Deploy a Gardenlet via Gardener Operator](deployment/deploy_gardenlet_via_operator.md)
  * [Scoped API Access for Gardenlets](deployment/gardenlet_api_access.md)
* [Overwrite image vector](deployment/image_vector.md)
* [Migration from Gardener `v0` to `v1`](deployment/migration_v0_to_v1.md)
* [Feature Gates in Gardener](deployment/feature_gates.md)
* [Configuring the Logging stack](deployment/configuring_logging.md)
* [SecretBinding Provider Controller](deployment/secret_binding_provider_controller.md)

## Operations

* [Gardener configuration and usage](operations/configuration.md)
* [Gardener Upgrade Guide](operations/upgrade-gardener.md)
* [Control Plane Migration](operations/control_plane_migration.md)
* [Enabling In-place Resource Updates](operations/enabling-in-place-resource-updates.md)
* [Immutable Backup Buckets](operations/immutable-backup-buckets.md)
* [Istio](operations/istio.md)
* [Kube API server load balancing](operations/kube_apiserver_loadbalancing.md)
* [`ManagedSeed`s: Register Shoot as Seed](operations/managed_seed.md)
* [`NetworkPolicy`s In Garden, Seed, Shoot Clusters](operations/network_policies.md)
* [Seed Bootstrapping](operations/seed_bootstrapping.md)
* [Seed Settings](operations/seed_settings.md)
* [Topology-Aware Traffic Routing](operations/topology_aware_routing.md)
* [Trusted TLS certificate for shoot control planes](operations/trusted-tls-for-control-planes.md)
* [Trusted TLS certificate for garden runtime cluster](operations/trusted-tls-for-garden-runtime.md)
* [Overlapping Network Ranges between Seeds and Shoots](operations/overlapping-network-ranges.md)
* [Disaster Recovery: Restoring a Garden Cluster to a new Runtime Cluster](operations/disaster_recovery_garden.md)

## Monitoring

* [Alerting](monitoring/alerting.md)
* [Connectivity](monitoring/connectivity.md)
* [Profiling Gardener Components](monitoring/profiling.md)
