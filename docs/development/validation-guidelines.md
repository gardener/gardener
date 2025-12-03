# Validation Guidelines

This document provides general developer guidelines on validation practices and conventions used in the Gardener codebase.
Developers and reviewers should consult this guide when writing, refactoring, and reviewing Gardener code.
If parts are unclear or new learnings arise, this guide should be adapted accordingly.

## The Importance of Validation

Validation is a critical part of maintaining the reliability, security, and overall quality of the Gardener project. It is important for several key reasons:
- **Security**
  - Validation protects against malicious inputs (like code injection), bypassing intended constraints, exploiting assumptions in the code and other attacks. The unexpected or harmful input is rejected before it can do damage.
- **API Consistency and Robustness**
  - Validating the resource ensures that incorrect or ambiguous data is caught early, preventing downstream controllers from acting on invalid input. Ensuring well-formed and valid API resource specifications is fundamental for successful lifecycle operations on these resources (for example successful Shoot cluster creation, consistent Shoot cluster state).
- **System Stability**
  - Invalid configurations passed unchecked into the system can result in unpredictable behavior or runtime errors. Early validation helps maintain system stability and predictability.
- **Clear and Immediate Feedback**
  - By catching errors in the admission phase of a request, the API users get fast, actionable feedback. The API users don't have to wait for an internal system operation to fail after several minutes to receive feedback that the provided input is invalid.

## Validation in Gardener

Kubernetes allows defining custom resources using [extension API server](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/apiserver-aggregation/) and [CustomResourceDefinitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).
Gardener is using both approaches.
The sections below [Validation for API resources](#validation-for-api-resources) and [Validation for CustomResourceDefinitions](#validation-for-customresourcedefinitions) describe how validation is handled for these two approaches.

### Validation for API Resources

[Gardener API server](../concepts/apiserver.md) is an extension API server.
An extension API server allows validation of a resource to be performed in the storage layer or in admission plugins.
The sections below [Validation in the Storage Layer](#validation-in-the-storage-layer) and [Validation in API Server Admission Plugins](#validation-in-api-server-admission-plugins) describe how validation is handled for these two approaches.

#### Validation in the Storage Layer

The API resources served by an API server (an extension API server or Kubernetes API server itself) provide a storage layer for a resource by implementing the [`k8s.io/apiserver/pkg/registry/rest.Storage`](https://pkg.go.dev/k8s.io/apiserver/pkg/registry/rest#Storage) interface.
These implementations usually embed the [`k8s.io/apiserver/pkg/registry/generic/registry.Store`](https://pkg.go.dev/k8s.io/apiserver/pkg/registry/generic/registry#Store) type in order to inherit CRUD semantics of a Kubernetes-like resource.
For reference, check out the storage layer implementation:
- for the Shoot resource - [`github.com/gardener/gardener/pkg/apiserver/registry/core/shoot/storage.REST`](https://pkg.go.dev/github.com/gardener/gardener/pkg/apiserver/registry/core/shoot/storage#REST)
- for the Pod resource - [`k8s.io/kubernetes/pkg/registry/core/pod/storage.REST`](https://pkg.go.dev/k8s.io/kubernetes/pkg/registry/core/pod/storage#REST).

The [`registry.Store`](https://pkg.go.dev/k8s.io/apiserver/pkg/registry/generic/registry#Store) type allows strategies for create, update and delete operations to be specified.
The resources provide a strategy for the storage layer by implementing interfaces like [`k8s.io/apiserver/pkg/registry/rest.RESTCreateStrategy`](https://pkg.go.dev/k8s.io/apiserver/pkg/registry/rest#RESTCreateStrategy) and [`k8s.io/apiserver/pkg/registry/rest.RESTUpdateStrategy`](https://pkg.go.dev/k8s.io/apiserver/pkg/registry/rest#RESTUpdateStrategy).
The `rest.RESTCreateStrategy` interface declares a `Validate` function, the `rest.RESTUpdateStrategy` interface - `ValidateUpdate` function.

For reference, check out the `Validate` and `ValidateUpdate` functions:
- for the Shoot resource - [`shoot.shootStrategy#Validate`](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/apiserver/registry/core/shoot/strategy.go#L174) and [`shoot.shootStrategy#ValidateUpdate`](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/apiserver/registry/core/shoot/strategy.go#L195)
- for the Pod resource - [`pod.podStrategy#Validate`](https://github.com/kubernetes/kubernetes/blob/5f829195e6ba30ca950e74f82c917b4cdbd05ecd/pkg/registry/core/pod/strategy.go#L113) and [`pod.podStrategy#ValidateUpdate`](https://github.com/kubernetes/kubernetes/blob/5f829195e6ba30ca950e74f82c917b4cdbd05ecd/pkg/registry/core/pod/strategy.go#L141).

The validation code itself for a resource resides in dedicated validation package. This validation package is API group specific and contains the validation code for all resources from the API group. These validation packages are consumed by the `Validate`/`ValidateUpdate` functions of the strategy implementation.
For reference, check out the validation packages:
- for the Gardener Core API group - [`github.com/gardener/gardener/pkg/apis/core/validation`](https://pkg.go.dev/github.com/gardener/gardener/pkg/apis/core/validation)
- for the Gardener Security API group - [`github.com/gardener/gardener/pkg/apis/security/validation`](https://pkg.go.dev/github.com/gardener/gardener/pkg/apis/security/validation)
- for the Kubernetes Core API group - [`k8s.io/kubernetes/pkg/apis/core/validation`](https://pkg.go.dev/k8s.io/kubernetes/pkg/apis/core/validation)

#### Writing Validation Code in the Storage Layer

- Write a validation code for a field in the storage layer if the validation logic does not require information from another resource. If the validation logic requires checking the presence of another resource or its specification, write the validation code in a validating API server admission plugin.
  - Example: Shoot's `.spec.kubernetes.kubeProxy.mode` field can be validated in the storage layer. The validation logic checks if the field value is one of the supported kube-proxy modes (`IPTables`, `IPVS`). There is no need to check another resource or its specification in order to validate this field.
- The `Validate` function of the strategy implementation **must** perform validation of an API resource. The `ValidateUpdate` function **must** perform validation specific for update operation (e.g. validate field is immutable) and **must** ensure the new object is valid (usually implemented by reusing the logic that `Validate` already uses).
  - Example: [`shoot.shootStrategy#Validate`](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/apiserver/registry/core/shoot/strategy.go#L174) invokes [`validation.ValidateShoot`](https://pkg.go.dev/github.com/gardener/gardener/pkg/apis/core/validation#ValidateShoot). [`shoot.shootStrategy#ValidateUpdate`](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/apiserver/registry/core/shoot/strategy.go#L195) invokes [`validation.ValidateShootUpdate`](https://pkg.go.dev/github.com/gardener/gardener/pkg/apis/core/validation#ValidateShootUpdate) which invokes [`validation.ValidateShoot`](https://pkg.go.dev/github.com/gardener/gardener/pkg/apis/core/validation#ValidateShoot) with the new object.

#### Validation in API Server Admission Plugins

An admission plugin (or admission controller) is a piece of code that intercepts requests to the API server prior to persistence of the resource, but after the request is authenticated and authorized. Check out [Admission Control in Kubernetes](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) to get an overview of the admission plugins in Kubernetes.

The admission plugins can validate, mutate, or do both. Mutating admission plugins may modify the data for the resource being modified; validating admission plugins may not.
The interfaces an admission plugin can implement are:
- [`k8s.io/apiserver/pkg/admission.ValidationInterface`](https://pkg.go.dev/k8s.io/apiserver/pkg/admission#ValidationInterface) - implemented by a validating admission plugin. The implementations must provide a `Validate` function.
- [`k8s.io/apiserver/pkg/admission.MutationInterface`](https://pkg.go.dev/k8s.io/apiserver/pkg/admission#MutationInterface) - implemented by a mutating admission plugin. The implementations must provide an `Admit` function.

A validating and mutating admission plugin implements both of them.
For reference, check out examples within Gardener for:
- a validating admission plugin: [`SeedValidator`](https://pkg.go.dev/github.com/gardener/gardener/plugin/pkg/seed/validator#Register)
- a mutating admission plugin: [`SeedMutator`](https://pkg.go.dev/github.com/gardener/gardener/plugin/pkg/seed/mutator#Register)
- a validating and mutating admission plugin: [`DeletionConfirmation`](https://pkg.go.dev/github.com/gardener/gardener/plugin/pkg/global/deletionconfirmation#Register)

The admission plugins are configurable. The API server has a set of admission plugins which are enabled by default. Additionally, the admission plugins can be enabled/disabled for an API server via the flags `--enable-admission-plugins`/`--disable-admission-plugins`.

An admission plugin can accept a configuration. The configuration is specified in API server admission configuration. See [Gardener API server admission configuration example](../../example/20-admissionconfig.yaml).
For reference, check out Gardener API server admission plugins that accepts configuration:
- [`ShootTolerationRestriction`](https://pkg.go.dev/github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction)
- [`ShootDNSRewriting`](https://pkg.go.dev/github.com/gardener/gardener/plugin/pkg/shoot/dnsrewriting)
- [`ShootResourceReservation`](https://pkg.go.dev/github.com/gardener/gardener/plugin/pkg/shoot/resourcereservation)

#### Writing Validation Code in API Server Admission Plugins

- Write the validation code for a field in an API server admission plugin if the validation logic requires checking the presence of another resource or its specification. Otherwise, write the validation code in the storage layer.
  - Example: Shoot's `.spec.region` field cannot be validated only in the storage layer. The validation logic for this field checks if the Shoot's CloudProfile supports the region in question. See the ([Shoot `.spec.region` validation](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/plugin/pkg/shoot/validator/admission.go#L1617-L1637)) in the [`ShootValidator` admission plugin](../concepts/apiserver-admission-plugins.md#shootvalidator). This is an example of validation logic that requires cross checking against another resource's specification. Hence, validation must be performed in an admission plugin.
- An admission plugin should embed the [`*admission.Handler`](https://pkg.go.dev/k8s.io/apiserver/pkg/admission#Handler) type. `admission.Handler` is a base type for admission plugins. When initializing the admission plugin, create a new handler with [`admission.Handler`](https://pkg.go.dev/k8s.io/apiserver/pkg/admission#NewHandler) and specify the operations the admission plugins supports. Example: [`SeedValidator` initialization](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/plugin/pkg/seed/validator/admission.go#L51-L56).
- An admission plugin should filter out resources and subresources it is not interested in. Example: [`SeedValidator` checks for resource and subresource](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/plugin/pkg/seed/validator/admission.go#L117-L125)
- An admission plugin should use listers for fetching resources.
  - The listers should be initialized by using the admission plugin initializer mechanism. An admission plugin should implement one of the predefined interfaces in [`github.com/gardener/gardener/pkg/apiserver/admission/initializer`](https://pkg.go.dev/github.com/gardener/gardener/pkg/apiserver/admission/initializer).
    - Example: `SeedValidator` has a `shootLister`. It obtains a `shootLister` by implementing the [`admissioninitializer.WantsCoreInformerFactory`](https://pkg.go.dev/github.com/gardener/gardener/pkg/apiserver/admission/initializer#WantsCoreInformerFactory) interface. The implementation must provide a `SetCoreInformerFactory` func which is used to initialize the `shootLister` ([`SeedValidator` `SetCoreInformerFactory` implementation](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/plugin/pkg/seed/validator/admission.go#L64-L70)).
  - The listers' initialization should be validated by implementing the `ValidateInitialization` function. Example: [`SeedValidator` `ValidateInitialization` implementation](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/plugin/pkg/seed/validator/admission.go#L80-L89).
  - An admission plugin should wait the cache of the informers (from which the listers are obtained from) to be synced. Example: [`SeedValidator` wait until cache sync](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/plugin/pkg/seed/validator/admission.go#L95-L109).
- Perform validation in a validating admission plugin. Do not use a mutating admission plugin for validation purposes. Example: https://github.com/gardener/gardener/pull/12786
- An admission plugin can only be added for resources served by the corresponding API server. Gardener API server can have an admission plugin only for resources it serves. For validation of resources served by the Kubernetes API server, use a validating webhook.

### Validation for CustomResourceDefinitions

The CustomResourceDefinitions (CRDs) allow validation to be specified using [OpenAPI v3.0 validation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation).
It is convenient to perform relatively simple validation checks.
For example, validate field is immutable or validate a slice contains at least one item.
However, more advanced validation is hard to express via these means and is performed by validating webhooks.

#### `extensions.gardener.cloud` API group

The extensibility story is based on the CRDs from the `extensions.gardener.cloud` API group.
They are validated by the `gardener-resource-manager`'s validating webhook. See [Extension Resource Validation](../concepts/resource-manager.md#extension-resource-validation).
The validation functions are defined in the [`github.com/gardener/gardener/pkg/apis/extensions/validation`](https://pkg.go.dev/github.com/gardener/gardener/pkg/apis/extensions/validation) package.

#### `operator.gardener.cloud` API group

The [gardener-operator](../concepts/operator.md) is responsible for the management of the garden cluster environment.
It does this by using the `Garden` and `Extension` CRDs from the `operator.gardener.cloud` API group.
The CRDs are validated by the `gardener-operator`'s validating webhook. See [`gardener-operator` validation section](../concepts/operator.md#validation).
The validation functions are defined in the [`github.com/gardener/gardener/pkg/apis/operator/v1alpha1/validation`](https://pkg.go.dev/github.com/gardener/gardener/pkg/apis/operator/v1alpha1/validation) package.

### Validation of Component Configurations

Most Gardener components have a component configuration that follows similar validation principles to those of the Gardener API.
Although the component configuration APIs are internal ones, they are also subject to validation.
For reference, check out the validation packages:
- for the `gardener-controller-manager` component configuration - [`github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1/validation`](https://pkg.go.dev/github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1/validation)
- for the `gardener-resource-manager` component configuration - [`github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1/validation`](https://pkg.go.dev/github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1/validation)

## Utility Functions

This section provides a collection of utility functions designed to simplify and standardize validation across the Gardener codebase.
These reusable helpers can be used to validate common data types, formats, and constraints — ensuring consistency, reducing code duplication, and improving overall reliability.

### Utility functions from Kubernetes

Kubernetes offers several packages that contain validation utilities:
- [`k8s.io/apimachinery/pkg/api/validation`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/validation), aliased as `apivalidation`
- [`k8s.io/apimachinery/pkg/apis/meta/v1/validation`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1/validation), aliased as `metav1validation`
- [`k8s.io/apimachinery/pkg/util/validation`](https://pkg.go.dev/k8s.io/apimachinery/pkg/util/validation)

Frequently used validation functions for creation:
- [`apivalidation.ValidateObjectMeta`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/validation#ValidateObjectMeta)
  - For validation in the storage layer, use this function in the `Validate` function of the strategy implementation to validate the object metadata of a resource.
  - It accepts one of the following `ValidateNameFunc`s:
    - [`apivalidation.NameIsDNSLabel`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/validation#NameIsDNSLabel)
      - a `ValidateNameFunc` that validates the name is a DNS 1123 label. See [RFC 1123 Label Names](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names).
    - [`apivalidation.NameIsDNSSubdomain`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/validation#NameIsDNSSubdomain)
      - a `ValidateNameFunc` that validates the name is a DNS subdomain. See [DNS Subdomain Names](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names).
- [`apivalidation.ValidateNonnegativeField`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/validation#ValidateNonnegativeField)
  - The validation for a non-negative integer or `time.Duration` is not unified. This is about to be addressed with https://github.com/gardener/gardener/issues/11285.
- [`apivalidation.ValidateAnnotations`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/validation#ValidateAnnotations)
  - Use this function only for annotations which are not part of the object metadata. `apivalidation.ValidateObjectMeta` already validates the object metadata annotations. See [example usage](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/apis/core/validation/shoot.go#L1927).
- [`metav1validation.ValidateLabels`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1/validation#ValidateLabels)
  - Use this function only for labels which are not part of the object metadata. `apivalidation.ValidateObjectMeta` already validates the object metadata labels. See [example usage](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/apis/core/validation/shoot.go#L1926).
- [`metav1validation.ValidateLabelSelector`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1/validation#ValidateLabelSelector)
  - Use this function to validate fields of type `metav1.LabelSelector`. See [example usage](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/apis/core/validation/shoot.go#L272-L274).
- [`apivalidation.ValidateNamespaceName`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/validation#ValidateNamespaceName), [`apivalidation.ValidateServiceAccountName`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/validation#ValidateServiceAccountName)
  - Use these functions to validate fields which represent Namespace name (Project's `.spec.namespace` field, CredentialsBinding's `.credentialsRef.namespace` field) or ServiceAccount name (ManagedSeed's `.spec.gardenlet.deployment.serviceAccountName`).
- [`validation.IsDNS1123Subdomain`](https://pkg.go.dev/k8s.io/apimachinery/pkg/util/validation#IsDNS1123Subdomain)
  - Validates that the value is a DNS subdomain. See [DNS Subdomain Names](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names).
- [`validation.IsValidIP`](https://pkg.go.dev/k8s.io/apimachinery/pkg/util/validation#IsValidIP)
  - Use this function to validate an IP address in canonical form.
  - A canonical form IP address standardizes how an IP address is written, ensuring consistent and unique representation.
  - A canonical form IPv4 address is in dotted-decimal notation with no leading zeros.
    - The IPv4 address `192.168.001.001` is non-canonical because it has leading zeros. The canonical form is `192.168.1.1`.
  - A canonical form IPv6 address uses the shortest possible representation.
    - The IPv6 address `2001:db8:f00:0:0:0:0:1` is non-canonical because it can be shortened by compressing the sequence of zero blocks. The canonical form is `2001:db8:f00::1`.
  - Use IP addresses in canonical form to ensure consistency, correctness and security. 
  - Use [`validation.IsValidIPForLegacyField`](https://pkg.go.dev/k8s.io/apimachinery/pkg/util/validation#IsValidIP) or [`net.ParseIP`](https://pkg.go.dev/net#ParseIP) to validate an IP address in non-canonical form.

Frequently used validation functions for update:
- [`apivalidation.ValidateObjectMetaUpdate`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/validation#ValidateObjectMetaUpdate)
  - For validation in the storage layer, use this function in the `ValidateUpdate` function of the strategy implementation to validate update to the object metadata of a resource.
- [`apivalidation.ValidateImmutableField`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/validation#ValidateImmutableField)
  - Use this function to validate field is immutable. See [example usage](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/apis/core/validation/shoot.go#L309).

### Utility functions from Gardener

Gardener offers several packages that contain validation utilities:
- [`github.com/gardener/gardener/pkg/utils/validation/cidr`](https://pkg.go.dev/github.com/gardener/gardener/pkg/utils/validation/cidr), aliased as `cidrvalidation`
  - The package provides validation utilities for CIDR ranges - validate if CIDR range is valid, if it overlaps with other CIDR ranges, etc.
  - The [`cidrvalidation.CIDR`](https://pkg.go.dev/github.com/gardener/gardener/pkg/utils/validation/cidr#CIDR) interface contains several utility functions for working with CIDR ranges. An implementation of the interface can be instantiated using the [`cidrvalidation.NewCIDR`](https://pkg.go.dev/github.com/gardener/gardener/pkg/utils/validation/cidr#NewCIDR) function.
  - The package also contains validation utilities for checking if Shoot networks intersect with Seed networks or if Shoot networks intersect between each other.
- [`github.com/gardener/gardener/pkg/utils/validation/kubernetes/core`](https://pkg.go.dev/github.com/gardener/gardener/pkg/utils/validation/kubernetes/core), aliased as `kubernetescorevalidation`
  - The package contains copy of several utility functions from [k8s.io/kubernetes/pkg/apis/core/validation](https://pkg.go.dev/k8s.io/kubernetes/pkg/apis/core/validation) and [k8s.io/kubernetes/pkg/apis/core/helper](https://pkg.go.dev/k8s.io/kubernetes/pkg/apis/core/helper). This is done to prevent importing the `k8s.io/kubernetes` dependency in Gardener.
- [`github.com/gardener/gardener/pkg/utils/validation`](https://pkg.go.dev/github.com/gardener/gardener/pkg/utils/validation), aliased as `validationutils`
  - The package provides validation utilities for component configuration APIs.

Frequently used validation functions:
- [`kubernetescorevalidation.ValidateResourceQuantityValue`](https://pkg.go.dev/github.com/gardener/gardener/pkg/utils/validation/kubernetes/core#ValidateResourceQuantityValue)
  - Use this function to validate fields of type `resource.Quantity`. See [example usage](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/apis/core/validation/shoot.go#L2240-L2242).
  - Use this function to validate fields of type `corev1.ResourceList`. See [example usage](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/apis/core/validation/shoot.go#L1392-L1407).
- [`kubernetescorevalidation.ValidateTolerations`](https://pkg.go.dev/github.com/gardener/gardener/pkg/utils/validation/kubernetes/core#ValidateTolerations)
  - Use this function to validate fields of type `[]corev1.Toleration`. See [example usage](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/apis/core/validation/seed.go#L156).
- [`validationutils.ValidateClientConnectionConfiguration`](https://pkg.go.dev/github.com/gardener/gardener/pkg/utils/validation#ValidateClientConnectionConfiguration)
  - The `componentbaseconfigv1alpha1.ClientConnectionConfiguration` type contains details for constructing a client against a Kubernetes cluster.
  - Use this function to validate fields of type `componentbaseconfigv1alpha1.ClientConnectionConfiguration` in component configuration APIs. See [example usage](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/controllermanager/apis/config/v1alpha1/validation/validation.go#L22).
- [`validationutils.ValidateLeaderElectionConfiguration`](https://pkg.go.dev/github.com/gardener/gardener/pkg/utils/validation#ValidateLeaderElectionConfiguration)
  - The `componentbaseconfigv1alpha1.LeaderElectionConfiguration` type defines the configuration of leader election clients for components that can run with leader election enabled.
  - Use this function to validate fields of type `componentbaseconfigv1alpha1.LeaderElectionConfiguration` in component configuration APIs. See [example usage](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/controllermanager/apis/config/v1alpha1/validation/validation.go#L23).

## Field-level Validation Errors

The [`k8s.io/apimachinery/pkg/util/validation/field`](https://pkg.go.dev/k8s.io/apimachinery/pkg/util/validation/field) package contains the predefined field-level errors.
These are commonly used by the validation logic to return detailed error information about specific fields in API resources.
Frequently used ones are:
- `field.Duplicate`
  - It indicates "duplicate value". This is used to report collisions of values that must be unique (e.g. names or IDs).
- `field.Forbidden`
  - It indicates "forbidden".  This is used to report valid (as per formatting rules) values which would be accepted under some conditions, but which are not permitted by current conditions (e.g. security policy).
- `field.InternalError`
  - It indicates "internal error". This is used to signal that an error was found that was not directly related to user input.
- `field.Invalid`
  - It indicates "invalid value". This is used to report malformed values (e.g. failed regex match, too long, out of bounds).
- `field.NotFound`
  - It indicates "value not found". This is used to report failure to find a requested value (e.g. looking up an ID).
- `field.NotSupported`
  - It indicates "unsupported value". This is used to report unknown values for enumerated fields (e.g. a list of valid values).
- `field.Required`
  -  It indicates "value required". This is used to report required values that are not provided (e.g. empty strings, null values, or empty arrays).

## General Guidelines

- **Validation is a strict requirement when contributing a new or enhancing existing API**.
  - If you add a new resource or if you add a new field to existing resource, make sure to add validation for it.
- Strictly follow the [Kubernetes API Conventions guides for validation](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#validation).
- When working with `ValidatingWebhookConfiguration`s, strictly follow the [Admission Webhook Good Practices](https://kubernetes.io/docs/concepts/cluster-administration/admission-webhooks-good-practices/).
- When working with `ValidatingWebhookConfiguration`s, ensure end-users cannot bypass the webhook by manipulating the resource labels.
  - A good practice for a webhook is to filter only for the resources it needs to validate by using an [`objectSelector`](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-objectselector).
  - Ensure end-users cannot bypass a validation by a webhook by removing resource label(s) which are used in the webhook's `objectSelector`.
  - Example: The [`ExtensionLabels` admission plugin](https://pkg.go.dev/github.com/gardener/gardener/plugin/pkg/global/extensionlabels#Register) maintains the `provider.extensions.gardener.cloud/<type>=true` label on CloudProfile, Seed, Shoot and other resources. The admission components of provider extensions use this label to filter for resources with the corresponding provider type (see [example usage](https://github.com/gardener/gardener-extension-provider-aws/blob/dd7dc94f049d7c8cf59f8211446f9015441d788c/pkg/admission/validator/webhook.go#L49-L51)). The `ExtensionLabels` admission plugin ensure the labels on `CREATE` and `UPDATE` requests. End-user cannot remove the `provider.extensions.gardener.cloud/<type>=true` label to bypass the provider extension admission webhook.
- Use [field-level validation errors](#field-level-validation-errors) according to their semantics.
  - Use `field.Required` for empty or null value; use `field.Invalid` for invalid value; use `field.Duplicate` for a duplicate value, etc. See [Field-level Validation Errors](#field-level-validation-errors).
- When introducing a new field, consider if it should be immutable or not.
  - Consider if updates to the field value should be allowed and are supported by the underlying controller. If not, consider making the field immutable. Add a doc string to the field to denote the immutability constraint.
- Introducing new validation for existing field or making existing validation more restrictive might be a breaking change.
  - When working with existing field, aim to add validation which is obvious and unlikely to break a working functionality.
  - If a breaking change is inevitable and it is likely to break a working functionality, consider the following alternatives:
    - Consider using "ratcheting" validation to incrementally tighten validation. See [Ratcheting validation](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api_changes.md#ratcheting-validation).
      - An example usage of "ratcheting" validation - allow invalid field value if the value is not updated; otherwise, do not allow. Example: https://github.com/gardener/gardener/pull/10664
    - Consider using a feature gate to roll out the breaking change. The feature gate gives control when to impose the breaking change. In case of issues, it is possible to revert back to the old behavior by disabling the feature gate.
    - In case the functionality is _relevant to the majority of the end-users_, consider imposing the breaking change only with the upcoming minor Kubernetes version. This way, end-users are forced to actively adapt their manifests when performing Kubernetes upgrades.
