// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terraformer

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// terraformer is a struct containing configuration parameters for the Terraform script it acts on.
//   - purpose is a one-word description depicting what the Terraformer does (e.g. 'infrastructure').
//   - namespace is the namespace in which the Terraformer will act.
//   - image is the Docker image name of the Terraformer image.
//   - ownerRef is the resource that owns the secrets and configmaps used by Terraformer
//   - configName is the name of the ConfigMap containing the main Terraform file ('main.tf').
//   - variablesName is the name of the Secret containing the Terraform variables ('terraform.tfvars').
//   - stateName is the name of the ConfigMap containing the Terraform state ('terraform.tfstate').
//   - envVars is a list of environment variables which will be injected in the resulting
//     Terraform pod. These variables can contain Terraform variables (i.e., must be prefixed
//     with TF_VAR_).
//   - configurationInitialized indicates whether the Terraform variables secret and Terraform script ConfigMap have been
//     successfully defined.
//   - stateInitialized indicates whether the Terraform state ConfigMap has been successfully defined.
//   - logLevel configures the log level for the Terraformer Pod (only compatible with terraformer@v2,
//     defaults to "info")
//   - terminationGracePeriodSeconds is the respective Pod spec field passed to Terraformer Pods.
//   - deadlineCleaning is the timeout to wait Terraformer Pods to be cleaned up.
//   - deadlinePod is the time to wait apply/destroy Pod to be completed.
type terraformer struct {
	logger       logr.Logger
	client       client.Client
	coreV1Client corev1client.CoreV1Interface

	purpose   string
	name      string
	namespace string
	image     string
	ownerRef  *metav1.OwnerReference

	configName    string
	variablesName string
	stateName     string
	envVars       []corev1.EnvVar

	configurationInitialized bool
	stateInitialized         bool

	logLevel                      string
	terminationGracePeriodSeconds int64

	deadlineCleaning    time.Duration
	deadlinePod         time.Duration
	deadlinePodCreation time.Duration

	// should only be disabled for testing
	useProjectedTokenMount bool
}

// RawState represent the terraformer state's raw data
type RawState struct {
	Data     string `json:"data"`
	Encoding string `json:"encoding"`
}

const (
	numberOfConfigResources = 3

	// ConfigSuffix is the suffix used for the ConfigMap which stores the Terraform configuration and variables declaration.
	ConfigSuffix = ".tf-config"

	// VariablesSuffix is the suffix used for the Secret which stores the Terraform variables definition.
	VariablesSuffix = ".tf-vars"

	// StateSuffix is the suffix used for the ConfigMap which stores the Terraform state.
	StateSuffix = ".tf-state"

	// Base64Encoding denotes base64 encoding for the RawState.Data
	Base64Encoding = "base64"

	// NoneEncoding denotes none encoding for the RawState.Data
	NoneEncoding = "none"
)

// Terraformer is the Terraformer interface.
type Terraformer interface {
	SetLogLevel(string) Terraformer
	SetEnvVars(envVars ...corev1.EnvVar) Terraformer
	SetTerminationGracePeriodSeconds(int64) Terraformer
	SetDeadlineCleaning(time.Duration) Terraformer
	SetDeadlinePod(time.Duration) Terraformer
	SetDeadlinePodCreation(time.Duration) Terraformer
	SetOwnerRef(*metav1.OwnerReference) Terraformer
	UseProjectedTokenMount(bool) Terraformer
	InitializeWith(ctx context.Context, initializer Initializer) Terraformer
	Apply(ctx context.Context) error
	Destroy(ctx context.Context) error
	GetRawState(ctx context.Context) (*RawState, error)
	GetState(ctx context.Context) ([]byte, error)
	IsStateEmpty(ctx context.Context) bool
	CleanupConfiguration(ctx context.Context) error
	RemoveTerraformerFinalizerFromConfig(ctx context.Context) error
	GetStateOutputVariables(ctx context.Context, variables ...string) (map[string]string, error)
	ConfigExists(ctx context.Context) (bool, error)
	NumberOfResources(ctx context.Context) (int, error)
	EnsureCleanedUp(ctx context.Context) error
	WaitForCleanEnvironment(ctx context.Context) error
}

// Initializer can initialize a Terraformer.
type Initializer interface {
	Initialize(ctx context.Context, config *InitializerConfig, ownerRef *metav1.OwnerReference) error
}

// Factory is a factory that can produce Terraformer and Initializer.
type Factory interface {
	NewForConfig(logger logr.Logger, config *rest.Config, purpose, namespace, name, image string) (Terraformer, error)
	New(logger logr.Logger, client client.Client, coreV1Client corev1client.CoreV1Interface, purpose, namespace, name, image string) Terraformer
	DefaultInitializer(c client.Client, main, variables string, tfVars []byte, stateInitializer StateConfigMapInitializer) Initializer
}

// StateConfigMapInitializer initialize terraformer state ConfigMap
type StateConfigMapInitializer interface {
	Initialize(ctx context.Context, c client.Client, namespace, name string, ownerRef *metav1.OwnerReference) error
}

// StateConfigMapInitializerFunc implements StateConfigMapInitializer
type StateConfigMapInitializerFunc func(ctx context.Context, c client.Client, namespace, name string, ownerRef *metav1.OwnerReference) error

// CreateOrUpdateState implements StateConfigMapInitializer.
// It use it field state for creating or updating the state ConfigMap
type CreateOrUpdateState struct {
	State *string
}
