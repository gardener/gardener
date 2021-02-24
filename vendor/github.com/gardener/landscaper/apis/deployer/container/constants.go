// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
)

// ContainerDeployerOperationForceCleanupAnnotation is the name of the annotation that triggers the force deletion of the deploy item.
// Force deletion means that the delete container is skipped and all other resources are cleaned up.
const ContainerDeployerOperationForceCleanupAnnotation = "container.deployer.landscaper.gardener.cloud/force-cleanup"

// ContainerDeployerFinalizer is the finalizer that is set by the container deployer
const ContainerDeployerFinalizer = "container.deployer.landscaper.gardener.cloud/finalizer"

// ContainerDeployerNameLabel is the name of the label that is used to identify managed pods.
const ContainerDeployerNameLabel = "container.deployer.landscaper.gardener.cloud/name"

// ContainerDeployerTypeLabel is a label that is used to identify secrets that contain the state of a container.
const ContainerDeployerTypeLabel = "container.deployer.landscaper.gardener.cloud/type"

// ContainerDeployerDeployItemNameLabel is the name of the label that is used to identify the deploy item of a pod.
const ContainerDeployerDeployItemNameLabel = "deployitem.container.deployer.landscaper.gardener.cloud/name"

// ContainerDeployerDeployItemNamespaceLabel is the name of the label that is used to identify the deploy item of a pod.
const ContainerDeployerDeployItemNamespaceLabel = "deployitem.container.deployer.landscaper.gardener.cloud/namespace"

// ContainerDeployerDeployItemGenerationLabel is the name of the label that indicates the deploy item generation.
const ContainerDeployerDeployItemGenerationLabel = "deployitem.container.deployer.landscaper.gardener.cloud/generation"

// InitContainerConditionType defines the condition for the current init container
const InitContainerConditionType = "InitContainer"

// WaitContainerConditionType defines the condition of the current wait container
const WaitContainerConditionType = "WaitContainer"

// OperationName is the name of the env var that specifies the current operation that the image should execute
const OperationName = "OPERATION"

// OperationType defines the value of a Operation that is propagated to the container.
type OperationType string

// OperationReconcile is the value of the Operation env var that defines a reconcile operation.
const OperationReconcile OperationType = "RECONCILE"

// OperationDelete is the value of the Operation env var that defines a delete operation.
const OperationDelete OperationType = "DELETE"

// BasePath is the base path inside a container that contains the container deployer specific data.
const BasePath = "/data/ls"

// SharedBasePath is the base path inside the container that is shared between the main and ls containers
var SharedBasePath = filepath.Join(BasePath, "shared")

// ImportsPathName is the name of the env var that points to the imports file.
const ImportsPathName = "IMPORTS_PATH"

// ImportsFilename is the name of the file that contains the import values as json.
const ImportsFilename = "import.json"

// ImportsPath is the path to the imports file.
var ImportsPath = filepath.Join(SharedBasePath, "imports", ImportsFilename)

// ExportsPathName is the name of the env var that points to the exports file.
const ExportsPathName = "EXPORTS_PATH"

// ExportsPath is the path to the export file.
var ExportsPath = filepath.Join(SharedBasePath, "exports", "values")

// ComponentDescriptorPathName is the name of the env var that points to the component descriptor.
const ComponentDescriptorPathName = "COMPONENT_DESCRIPTOR_PATH"

// ComponentDescriptorPath is the path to the component descriptor file.
var ComponentDescriptorPath = filepath.Join(SharedBasePath, "component_descriptor.json")

// ContentPathName is the name of the env var that points to the blob content of the definition.
const ContentPathName = "CONTENT_PATH"

// ContentPath is the path to the content directory.
var ContentPath = filepath.Join(SharedBasePath, "content")

// StatePathName is the name of the env var that points to the directory where the state can be stored.
const StatePathName = "STATE_PATH"

// StatePath is the path to the state directory.
var StatePath = filepath.Join(SharedBasePath, "state")

// ConfigurationPathName is the name of the env var that points to the provider configuration file.
const ConfigurationPathName = "CONFIGURATION_PATH"

// ConfigurationFilename is the name of the file that contains the provider configuration as json.
const ConfigurationFilename = "configuration.json"

// ConfigurationPath is the path to the configuration file.
var ConfigurationPath = filepath.Join(BasePath, "internal", ConfigurationFilename)

// RegistrySecretBasePath is the path to all OCI pull secrets
var RegistrySecretBasePath = filepath.Join(BasePath, "registry_secrets")

// RegistrySecretBasePathName is the environment variable pointing to the file system location of all OCI pull secrets
const RegistrySecretBasePathName = "REGISTRY_SECRETS_DIR"

// PodName is the name of the env var that contains the name of the pod.
const PodName = "POD_NAME"

// PodNamespaceName is the name of the env var that contains the namespace of the pod.
const PodNamespaceName = "POD_NAMESPACE"

// DeployItemName is the name of the env var that contains name of the source DeployItem.
const DeployItemName = "DEPLOY_ITEM_NAME"

// DeployItemNamespaceName is the name of the env var that contains namespace of the source DeployItem.
const DeployItemNamespaceName = "DEPLOY_ITEM_NAMESPACE"

// MainContainerName is the name of the container running the user workload.
const MainContainerName = "main"

// InitContainerName is the name of the container running the init container.
const InitContainerName = "init"

// WaitContainerName is the name of the container running the sidecar container.
const WaitContainerName = "wait"

// ContainerDeployerStateUUIDAnnotation is a annotation that is used to group chunks
// that are stored in the secrets.
const ContainerDeployerStateUUIDAnnotation = "container.deployer.landscaper.gardener.cloud/uuid"

// ContainerDeployerStateNumAnnotation is a annotation that is used to define the order of chunks
// that are stored in the secrets.
const ContainerDeployerStateNumAnnotation = "container.deployer.landscaper.gardener.cloud/num"

var (
	DefaultEnvVars = []corev1.EnvVar{
		{
			Name:  ImportsPathName,
			Value: ImportsPath,
		},
		{
			Name:  ExportsPathName,
			Value: ExportsPath,
		},
		{
			Name:  ComponentDescriptorPathName,
			Value: ComponentDescriptorPath,
		},
		{
			Name:  ContentPathName,
			Value: ContentPath,
		},
		{
			Name:  StatePathName,
			Value: StatePath,
		},
		{
			Name: PodName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: PodNamespaceName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}
)
