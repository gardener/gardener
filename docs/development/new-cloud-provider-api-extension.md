# API Extension and Validation for New Cloud Provider

This document describes how to extend Gardener to support a new cloud provider in the API. There is a significant number of steps necessary.

## Add Cloud Provider Support

Cloud provider support is headed through the following major steps:

1. Add the necessary structures, logic and tests to the code
2. Update example templates
3. Generate auto-generated code
4. Generate examples

The detailed steps are as follows:

1. Add the cloud provider specific configuration to the template files in [hack/templates/resources](../../hack/templates/resources/) :
  * `CloudProfile` [30-cloudprofile.yaml.tpl](../../hack/templates/resources/30-cloudprofile.yaml.tpl) This is the definition of a profile for this provider.
  * `Secret` [40-secret-seed.yaml.tpl](../../hack/templates/resources/40-secret-seed.yaml.tpl) This contains the secrets necessary to invoke the cloud provider's API to create blob storage buckets for managing the etcd backups of Shoot clusters, and the kubeconfig to access the Seed cluster. 
  * `Seed`: [50-seed.yaml.tpl](../../hack/templates/resources/50-seed.yaml.tpl) This contains the configuration for the Seed cluster.
  * `Secret`: [70-secret-cloudprovider.yaml.tpl](../../hack/templates/70-secret-cloudprovider.yaml.tpl) This contains the credentials necessary for to invoke the cloud provider's API to deploy the Shoot cluster(s).
  * `SecretBinding`: [80-secretbinding-cloudprovider.yaml.tpl](../../hack/templates/resources/80-secretbinding-cloudprovider.yaml.tpl) This binds the secret defined in the previous step. Ensure that the `secretRef` references the named secret from the previous step.
  * `Shoot`: [90-shoot.yaml.tpl](../../hack/templates/resources/90-shoot.yaml.tpl) This creates an actual Shoot in the given provider, using the `Secret` and `CloudProfile` created earlier.
2. Add the cloud provider to [hack/generate-examples](../../hack/generate-examples) 
3. Run [hack/generate-examples](../../hack/generate-examples) to generate the examples in [example](../../example/)
4. Update [pkg/apis/garden/helper/helpers.go](../../pkg/apis/garden/helper/helpers.go) to include the cloud provider at the following key points:
    * Select the cloud provider in `DetermineCloudProviderInProfile()`
    * Return the cloud provider in the error at the end of `DetermineCloudProviderInProfile()` if the number of clouds is not 1
    * Select the cloud provider in `DetermineCloudProviderInShoot()`
    * Return the cloud provider in the error at the end of `DetermineCloudProviderInShoot()` if the number of clouds is not 1
5. Update [pkg/apis/garden/types.go](../../pkg/apis/garden/types.go) to include the cloud provider at the following key points:
    * Define the name of the cloud provider as a key of `CloudProfileSpec struct`, where the value is a pointer to `<provider>Profile`, e.g. `AWS *AWSProfile`. See the existing examples
    * Define the struct for the `<provider>Profile`. See the existing examples. The `Profile` contains all of the properties necessary to call the cloud provider, including constraints, image information, machine type and volume type.
    * Define the name of the cloud provider as a key of `Cloud struct`, where the value is a pointer to `<provider>Cloud`, e.g. `AWS *AWSCloud`. 
    * Define the struct for the `<providerCloud`. See the existing examples. The `Cloud` contains the Shoot specification for the specific deployed Shoot to the given provider.
    * Define a constant `string` alias for the cloud provider as `CloudProvider<provider> CloudProvider = "alias"`. See the existing examples.
6. Update [pkg/apis/garden/v1beta1/defaults.go](../../pkg/apis/garden/v1beta1/defaults.go) to provide defaults for the cloud provider. See the existing examples.
7. Update [pkg/apis/garden/v1beta1/helper/helpers.go](../../pkg/apis/garden/v1beta1/helper/helpers.go) to include the cloud provider at the following key points:
    * Select the cloud provider in `DetermineCloudProviderInProfile()`
    * Return the cloud provider in the error at the end of `DetermineCloudProviderInProfile()` if the number of clouds is not 1
    * Select the cloud provider in `DetermineCloudProviderInShoot()`
    * Return the cloud provider in the error at the end of `DetermineCloudProviderInShoot()` if the number of clouds is not 1
    * Return the appropriate image name for the cloud provider in `DetermineMachineImage()`
    * Return the latest available kubernetes version in `DetermineLatestKubernetesVersion()` 
8. Update [pkg/apis/garden/v1beta1/types.go](../../pkg/apis/garden/v1beta1/types.go) to include the cloud provider at the following key points:
    * Define the name of the cloud provider as a key of `CloudProfileSpec struct`, where the value is a pointer to `<provider>Profile`. Be sure to include json tags on the struct definition.
    * Define the struct for `<provider>Profile`. See the existing examples. The `Profile` contains all of the properties necessary to call the cloud provider, including constraints.
9. Update [pkg/apis/garden/validation/validation.go](../../pkg/apis/garden/validation/validation.go) to include the cloud provider at the following key points:
    * Include the optional cloud provider DNS, if defined, in `availableDNS` in `func init()`
    * Include the cloud provider in the error message in `ValidateCloudProfileSpec()` if there was an error returned from `helper.DetermineCloudProviderInProfile`.
    * Validate all of the necessary elements for the cloud provider in `ValidateCloudProfileSpec()`, and add all the errors to `allErrs`. See the existing examples.
    * Write validation functions for the cloud provider for each of the elements validates in the previous step. See the existing examples.
    * In `validateCloud()`, create validation logic for the cloud provider. See existing examples. The net result should be any errors add to `allErrs`.
    * In `ValidateShootSpecUpdate()`, validate immutable fields are not changed. See existing examples.
10. Create validation tests in [pkg/apis/garden/validation/validation_test.go](../../pkg/apis/garden/validation/validation_test.go) . See existing examples.
11. Run [hack/generate-code](../../hack/generate-code) to generate the necessary code.

## Add Validations

Validations check that parameters submitted to Gardener for the given cloud profile are valid. It has two major steps:

1. Write validators
2. Write tests for the validators

The details are as follows:

1. Update [plugin/pkg/shoot/quotavalidator/admission.go](../../plugin/pkg/shoot/quotavalidator/admission.go) at the following key points:
    * Add logic in `getShootWorkerResources()` to populate the return `[]quotaworker` slice with the workers for the cloud provider.
    * Add logic in `getMachineTypes()` to return the list of machine types available for the cloud provider.
    * Add logic in `getVolumeTypes()` to return the list of volume types available for the cloud provider.
    * Add logic in `quotaVerificationNeeded()` to return if quota verification is required. A Shoot cluster can have an optional quota, maximum resources that it can consume. This checks if a quota-limited Shoot cluster has changes in its specification, and thus whether a change is allowed.
2. Update [plugin/pkg/shoot/quotavalidator/admission_test.go](../../plugin/pkg/shoot/quotavalidator/admission_test.go) to test logic from the previous step.
3. Update [plugin/pkg/shoot/validator/admission.go](../../plugin/pkg/shoot/validator/admission.go) at the following key points:
    * Add logic to `Admit()` where other cloud providers exist to populate the keys of `&garden.Cloud{}` with the `MachineImage` for the cloud provider. See existing examples.
    * Add logic to `switch cloudProviderInShoot` to add a case for the cloud provider, validating that it can get the machine image. The logic here should be minimal, instead calling out to a function that does more detailed validation, e.g. `validateAWS()` or `validateAzure()`, i.e. `validate<Provider>()`.
    * Create the function `validate<Provider>()` that validates all the elements of the request, including sub-functions, if necessary. See existing examples.
4. Update [plugin/pkg/shoot/validator/admission_test.go](../../plugin/pkg/shoot/validator/admission_test.go) to test logic from the previous step.

