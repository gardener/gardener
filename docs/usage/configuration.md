# Gardener Configuration and Usage

Gardener automates the full lifecycle of Kubernetes clusters as a service.
Additionally, it has several extension points allowing external controllers to plug-in to the lifecycle.
As a consequence, there are several configuration options for the various custom resources that are partially required.

This document describes the

1. [configuration and usage of Gardener as operator/administrator](#configuration-and-usage-of-gardener-as-operatoradministrator).
2. [configuration and usage of Gardener as end-user/stakeholder/customer](#configuration-and-usage-of-gardener-as-end-userstakeholdercustomer).

## Configuration and Usage of Gardener as Operator/Administrator

When we use the terms "operator/administrator" we refer to both the people deploying and operating Gardener.
Gardener consists out of three components:

1. `gardener-apiserver`, a Kubernetes-native API extension that serves custom resources in the Kubernetes-style (like `Seed`s and `Shoot`s), and a component that contains multiple admission plugins.
1. `gardener-controller-manager`, a component consisting out of multiple controllers that implement reconciliation and deletion flows for the various custom resources (e.g., it contains the logic for reconciliation and deletion of `Shoot`s).
1. `gardener-scheduler`, a component that assigns newly created `Shoot` clusters to appropriate `Seed` clusters.

Each of these components have various configuration options.
The `gardener-apiserver` uses the standard API server library maintained by the Kubernetes community, and as such it mainly supports command line flags.
The two other components are using so-called componentconfig files that describe their configuration in a Kubernetes-style versioned object.

### Configuration file for Gardener controller manager

The Gardener controller manager does only support one command line flag which should be a path to a valid configuration file.
Please take a look at [this](../../example/20-componentconfig-gardener-controller-manager.yaml) example configuration.

### Configuration file for Gardener scheduler

The Gardener scheduler also only supports one command line flag which should be a path to a valid scheduler configuration file.
Please take a look at [this](../../example/20-componentconfig-gardener-scheduler.yaml) example configuration.
Information about the concepts of the gardener scheduler can be found [here](./scheduler.md)

### System configuration

After successful deployment of the three components you need to setup the system.
Let's first focus on some "static" configuration.
When the `gardener-controller-manager` starts it scans the `garden` namespace of the garden cluster for `Secret`s that have influence on its reconciliation loops, mainly the `Shoot` reconciliation:

* **Internal domain secret**, contains the DNS provider credentials (having appropriate privileges) which will be used to create/delete so-called "internal" DNS records for the Shoot clusters, please see [this](../../example/10-secret-internal-domain.yaml) for an example.
  * This secret is used in order to establish a stable endpoint for shoot clusters which is used internally by all control plane components.
  * The DNS records are normal DNS records but called "internal" in our scenario because only the kubeconfigs for the control plane components use this endpoint when talking to the shoot clusters.
  * It is forbidden to change the internal domain secret if there are existing shoot clusters.

* **Default domain secrets** (optional), contain the DNS provider credentials (having appropriate privileges) which will be used to create/delete DNS records for a default domain for shoots (e.g., `example.com`), please see [this](../../example/10-secret-default-domain.yaml) for an example.
  * Not every end-user/stakeholder/customer has its own domain, however, Gardener needs to create a DNS record for every shoot cluster.
  * As landscape operator you might want to define a default domain owned and controlled by you that is used for all shoot clusters that don't specify their own domain.

:warning: Please note that the mentioned domain secrets are only needed if you have at least one seed cluster that is not tainted with `seed.gardener.cloud/disable-dns`.
Seeds with this taint don't create any DNS records for shoots scheduled on it, hence, if you only have such seeds, you don't need to create the domain secrets.

* **Alerting secrets** (optional), contain the alerting configuration and credentials for the [Alertmanager](https://prometheus.io/docs/alerting/alertmanager/) to send email alerts. It is also possible to configure the monitoring stack to send alerts to an alertmanager not deployed by Gardener to handle alerting. Please see [this](../../example/10-secret-alerting.yaml) for an example.
  * If email alerting is configured:
    * An Alertmanager is deployed into each seed cluster that handles the alerting for all shoots on the seed cluster.
    * Gardener will inject the SMTP credentials into the configuration of the Alertmanager.
    * The Alertmanager will send emails to the configured email address in case any alerts are firing.
  * If an external alertmanager is configured:
    * Each shoot has a [Prometheus](https://prometheus.io/docs/introduction/overview/) responsible for monitoring components and sending out alerts. The alerts will be sent to a URL configured in the alerting secret.
    * This external alertmanager is not managed by Gardener and can be configured however the operator sees fit.
    * Supported authentication types are no authentication, basic, or mutual TLS.

* **OpenVPN Diffie-Hellmann Key secret** (optional), contains the self-generated Diffie-Hellmann key used by OpenVPN in your landscape, please see [this](../../example/10-secret-openvpn-diffie-hellman.yaml) for an example.
  * If you don't specify a custom key then a default key is used, but for productive landscapes it's recommend to create a landscape-specific key and define it.

* **Global monitoring secrets** (optional), contains basic authentication credentials for the Prometheus aggregating metrics for all clusters.
  * These secrets are synced to each seed cluster and used to gain access to the aggregate monitoring components.

Apart from this "static" configuration there are several custom resources extending the Kubernetes API and used by Gardener.
As an operator/administrator you have to configure some of them to make the system work.

### `CloudProfile`s

`CloudProfile`s are resources that describe a specific environment of an underlying infrastructure provider, e.g. AWS, Azure, etc.
Each shoot has to reference a `CloudProfile` to declare the environment it should be created in.
In a `CloudProfile` you specify certain constraints like available machine types, regions, which Kubernetes versions you want to offer, etc.
End-users can read `CloudProfile`s to see these values, but only operators can change the content or create/delete them.
When a shoot is created or updated then an admission plugin checks that only values are used that are allowed via the referenced `CloudProfile`.

Additionally, a `CloudProfile` may contain a `providerConfig` which is a special configuration dedicated for the infrastructure provider.
Gardener does not evaluate or understand this config, but extension controllers might need for declaration of provider-specific constraints, or global settings.

Please see [this](../../example/30-cloudprofile.yaml) example manifest and consult the documentation of your provider extension controller to get information about its `providerConfig`.

### `Seed`s

`Seed`s are resources that represent seed clusters.
Gardener does not care about how a seed cluster got created - the only requirement is that it is of at least Kubernetes v1.11 and passes the Kubernetes conformance tests.
You have to provide the seed's kubeconfig inside a secret that is referenced by the `Seed` resource.

Please see [this](../../example/40-secret-seed.yaml), [this](../../example/40-secret-seed-backup.yaml), and [this](../../example/50-seed.yaml) example manifest.

### `Quota`s

In order to allow end-user not having their own dedicated infrastructure account to try out Gardener you can register an account owned by you that you use for trial clusters.
Trial clusters can be put under quota such that they don't consume too many resources (resulting in costs), and so that one user cannot consume all resources on his own.
These clusters are automatically terminated after a specified time, but end-users may extend the lifetime manually if needed.

Please see [this](../../example/60-quota.yaml) example manifest.

## Configuration and Usage of Gardener as End-User/Stakeholder/Customer

As an end-user/stakeholder/customer you are using a Gardener landscape that has been setup for you by another team.
You don't need to care about how Gardener itself has to be configured or how it has to be deployed.

### `Project`s

The first thing before creating a shoot cluster is to create a `Project`.
A project is used to group multiple shoot clusters together.
You can invite colleagues to the project to enable collaboration, and you can either make them `admin` or `viewer`.
After you have created a project you will get a dedicated namespace in the garden cluster for all your shoots.

Please see [this](../../example/05-project-dev.yaml) example manifest.

### `SecretBinding`s

Now that you have a namespace the next step is registering your infrastructure provider account.

Please see [this](../../example/70-secret-provider.yaml) example manifest and consult the documentation of the extension controller for your infrastructure provider to get information about which keys are required in this secret.

After the secret has been created you have to create a special `SecretBinding` resource that binds this secret.
Later when creating shoot clusters you will reference such a binding.

Please see [this](../../example/80-secretbinding.yaml) example manifest.

### `Shoot`s

Shoot cluster contain various settings that influence how your Kubernetes cluster will look like in the end.
As Gardener heavily relies on extension controllers for operating system configuration, networking, and infrastructure specifics, you have the possibility (and duty) to provide these provider-specific configurations as well.
Such configurations are not evaluated by Gardener (because it doesn't know/understand them), but they are only transported to the respective extension controller.

:warning: This means that any configuration issues/mistake on your side that relates to a provider-specific flag or setting cannot be caught during the update request itself but only later during the reconciliation.

Please see [this](../../example/90-shoot.yaml) example manifest and consult the documentation of the provider extension controller to get information about its `spec.provider.controlPlaneConfig`, `.spec.provider.infrastructureConfig`, and `.spec.provider.workers[].providerConfig`.

### `(Cluster)OpenIDConnectPreset`s

Please see [this](./openidconnect-presets.md) separate documentation file.
