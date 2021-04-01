// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package core

const (
	// LandscapeConfigName is the namespace unique name of the landscape configuration
	LandscapeConfigName = "default"

	// DataObjectSecretDataKey is the key of the secret where the landscape and installations stores their merged configuration.
	DataObjectSecretDataKey = "config"

	// LandscaperFinalizer is the finalizer of the landscaper
	LandscaperFinalizer = "finalizer.landscaper.gardener.cloud"

	// Annotations

	// OperationAnnotation is the annotation that specifies a operation for a component
	OperationAnnotation = "landscaper.gardener.cloud/operation"

	// Labels

	// LandscaperComponentLabelName is the name of the labels the holds the information about landscaper components.
	// This label should be set on landscaper related components like the landscaper controller or deployers.
	LandscaperComponentLabelName = "landscaper.gardener.cloud/component"

	// Component Descriptor

	// BlueprintType is the name of the blueprint type in a component descriptor.
	BlueprintType = "landscaper.gardener.cloud/blueprint"

	// OldBlueprintType is the old name of the blueprint type in a component descriptor.
	OldBlueprintType = "blueprint"

	// BlueprintFileName is the filename of a component definition on a local path
	BlueprintFileName = "blueprint.yaml"

	// BlueprintArtifactsMediaType is the reserved media type for a blueprint that is stored as its own artifact.
	BlueprintArtifactsMediaType = "application/vnd.gardener.landscaper.blueprint.v1+tar+gzip"

	// InlineComponentDescriptorLabel is the label name used for nested inline component descriptors
	InlineComponentDescriptorLabel = "landscaper.gardener.cloud/component-descriptor"

	//
	JSONSchemaResourceType = "landscaper.gardener.cloud/jsonschema"

	JSONSchemaArtifactMediaType = "application/vnd.gardener.landscaper.jsonscheme.v1+json"
)
