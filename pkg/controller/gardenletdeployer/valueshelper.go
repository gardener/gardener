// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenletdeployer

import (
	"os"
	"strings"

	"k8s.io/component-base/version"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

// ValuesHelper provides methods for merging GardenletDeployment and GardenletConfiguration with parent,
// as well as computing the values to be used when applying the gardenlet chart.
type ValuesHelper interface {
	// MergeGardenletDeployment merges the given GardenletDeployment with the values from the parent gardenlet.
	MergeGardenletDeployment(*seedmanagementv1alpha1.GardenletDeployment) (*seedmanagementv1alpha1.GardenletDeployment, error)
	// MergeGardenletConfiguration merges the given GardenletConfiguration with the parent GardenletConfiguration.
	MergeGardenletConfiguration(config *gardenletconfigv1alpha1.GardenletConfiguration) (*gardenletconfigv1alpha1.GardenletConfiguration, error)
	// GetGardenletChartValues computes the values to be used when applying the gardenlet chart.
	GetGardenletChartValues(*seedmanagementv1alpha1.GardenletDeployment, *gardenletconfigv1alpha1.GardenletConfiguration, string) (map[string]any, error)
}

// valuesHelper is a concrete implementation of ValuesHelper
type valuesHelper struct {
	config *gardenletconfigv1alpha1.GardenletConfiguration
}

// NewValuesHelper creates a new ValuesHelper with the given parent GardenletConfiguration and image vector.
func NewValuesHelper(config *gardenletconfigv1alpha1.GardenletConfiguration) ValuesHelper {
	return &valuesHelper{
		config: config,
	}
}

// MergeGardenletDeployment merges the given GardenletDeployment with the values from the parent gardenlet.
func (vp *valuesHelper) MergeGardenletDeployment(deployment *seedmanagementv1alpha1.GardenletDeployment) (*seedmanagementv1alpha1.GardenletDeployment, error) {
	// Convert deployment object to values
	deploymentValues, err := utils.ToValuesMap(deployment)
	if err != nil {
		return nil, err
	}

	// Get parent deployment values
	parentDeployment, err := getParentGardenletDeployment()
	if err != nil {
		return nil, err
	}
	parentDeploymentValues, err := utils.ToValuesMap(parentDeployment)
	if err != nil {
		return nil, err
	}

	// Merge with parent
	deploymentValues = utils.MergeMaps(parentDeploymentValues, deploymentValues)

	// Convert deployment values back to an object
	var deploymentObj *seedmanagementv1alpha1.GardenletDeployment
	if err := utils.FromValuesMap(deploymentValues, &deploymentObj); err != nil {
		return nil, err
	}

	return deploymentObj, nil
}

// MergeGardenletConfiguration merges the given GardenletConfiguration with the parent GardenletConfiguration.
func (vp *valuesHelper) MergeGardenletConfiguration(config *gardenletconfigv1alpha1.GardenletConfiguration) (*gardenletconfigv1alpha1.GardenletConfiguration, error) {
	// Convert configuration object to values
	configValues, err := utils.ToValuesMap(config)
	if err != nil {
		return nil, err
	}

	// Get parent config values
	parentConfigValues, err := utils.ToValuesMap(vp.config)
	if err != nil {
		return nil, err
	}

	// Delete gardenClientConnection.bootstrapKubeconfig, seedClientConnection.kubeconfig, and seedConfig in parent config values
	parentConfigValues, err = utils.DeleteFromValuesMap(parentConfigValues, "gardenClientConnection", "bootstrapKubeconfig")
	if err != nil {
		return nil, err
	}
	parentConfigValues, err = utils.DeleteFromValuesMap(parentConfigValues, "seedClientConnection", "kubeconfig")
	if err != nil {
		return nil, err
	}
	parentConfigValues, err = utils.DeleteFromValuesMap(parentConfigValues, "seedConfig")
	if err != nil {
		return nil, err
	}

	// Merge with parent
	configValues = utils.MergeMaps(parentConfigValues, configValues)

	// Convert config values back to an object
	var configObj *gardenletconfigv1alpha1.GardenletConfiguration
	if err := utils.FromValuesMap(configValues, &configObj); err != nil {
		return nil, err
	}

	return configObj, nil
}

// GetGardenletChartValues computes the values to be used when applying the gardenlet chart.
func (vp *valuesHelper) GetGardenletChartValues(
	deployment *seedmanagementv1alpha1.GardenletDeployment,
	config *gardenletconfigv1alpha1.GardenletConfiguration,
	bootstrapKubeconfig string,
) (map[string]any, error) {
	var err error

	// Get deployment values
	deploymentValues, err := vp.getGardenletDeploymentValues(deployment)
	if err != nil {
		return nil, err
	}

	// Get config values
	configValues, err := vp.getGardenletConfigurationValues(config, bootstrapKubeconfig)
	if err != nil {
		return nil, err
	}

	// Set gardenlet values to deployment values
	gardenletValues := deploymentValues

	// Ensure gardenlet values is a non-nil map
	gardenletValues = utils.InitValuesMap(gardenletValues)

	// Set config values in gardenlet values
	return utils.SetToValuesMap(gardenletValues, configValues, "config")
}

// getGardenletDeploymentValues computes and returns the gardenlet deployment values from the given GardenletDeployment.
func (vp *valuesHelper) getGardenletDeploymentValues(deployment *seedmanagementv1alpha1.GardenletDeployment) (map[string]any, error) {
	// Convert deployment object to values
	deploymentValues, err := utils.ToValuesMap(deployment)
	if err != nil {
		return nil, err
	}

	// make sure map is initialized
	deploymentValues = utils.InitValuesMap(deploymentValues)

	// Set imageVectorOverwrite and componentImageVectorOverwrites from parent
	parentImageVectorOverwrite, err := getParentImageVectorOverwrite()
	if err != nil {
		return nil, err
	}

	if parentImageVectorOverwrite != nil {
		deploymentValues["imageVectorOverwrite"] = *parentImageVectorOverwrite
	}

	parentComponentImageVectorOverwrites, err := getParentComponentImageVectorOverwrites()
	if err != nil {
		return nil, err
	}

	if parentComponentImageVectorOverwrites != nil {
		deploymentValues["componentImageVectorOverwrites"] = *parentComponentImageVectorOverwrites
	}

	return deploymentValues, nil
}

// getGardenletConfigurationValues computes and returns the gardenlet configuration values from the given GardenletConfiguration.
func (vp *valuesHelper) getGardenletConfigurationValues(config *gardenletconfigv1alpha1.GardenletConfiguration, bootstrapKubeconfig string) (map[string]any, error) {
	// Convert configuration object to values
	configValues, err := utils.ToValuesMap(config)
	if err != nil {
		return nil, err
	}

	// If bootstrap kubeconfig is specified, set it in gardenClientConnection
	// Otherwise, if kubeconfig path is specified in gardenClientConnection, read it and store its contents
	if bootstrapKubeconfig != "" {
		configValues, err = utils.SetToValuesMap(configValues, bootstrapKubeconfig, "gardenClientConnection", "bootstrapKubeconfig", "kubeconfig")
		if err != nil {
			return nil, err
		}
	} else {
		kubeconfigPath, err := utils.GetFromValuesMap(configValues, "gardenClientConnection", "kubeconfig")
		if err != nil {
			return nil, err
		}
		if kubeconfigPath != nil && kubeconfigPath.(string) != "" {
			kubeconfig, err := os.ReadFile(kubeconfigPath.(string))
			if err != nil {
				return nil, err
			}
			configValues, err = utils.SetToValuesMap(configValues, string(kubeconfig), "gardenClientConnection", "kubeconfig")
			if err != nil {
				return nil, err
			}
		}
	}

	// If kubeconfig path is specified in seedClientConnection, read it and store its contents
	kubeconfigPath, err := utils.GetFromValuesMap(configValues, "seedClientConnection", "kubeconfig")
	if err != nil {
		return nil, err
	}
	if kubeconfigPath != nil && kubeconfigPath.(string) != "" {
		kubeconfig, err := os.ReadFile(kubeconfigPath.(string))
		if err != nil {
			return nil, err
		}
		configValues, err = utils.SetToValuesMap(configValues, string(kubeconfig), "seedClientConnection", "kubeconfig")
		if err != nil {
			return nil, err
		}
	}

	// Read server certificate file and store its contents
	certPath, err := utils.GetFromValuesMap(configValues, "server", "https", "tls", "serverCertPath")
	if err != nil {
		return nil, err
	}
	if certPath != nil && certPath.(string) != "" && !strings.Contains(certPath.(string), secrets.TemporaryDirectoryForSelfGeneratedTLSCertificatesPattern) {
		cert, err := os.ReadFile(certPath.(string))
		if err != nil {
			return nil, err
		}
		configValues, err = utils.SetToValuesMap(configValues, string(cert), "server", "https", "tls", "crt")
		if err != nil {
			return nil, err
		}
	}

	// Read server key file and store its contents
	keyPath, err := utils.GetFromValuesMap(configValues, "server", "https", "tls", "serverKeyPath")
	if err != nil {
		return nil, err
	}
	if keyPath != nil && keyPath.(string) != "" && !strings.Contains(keyPath.(string), secrets.TemporaryDirectoryForSelfGeneratedTLSCertificatesPattern) {
		key, err := os.ReadFile(keyPath.(string))
		if err != nil {
			return nil, err
		}
		configValues, err = utils.SetToValuesMap(configValues, string(key), "server", "https", "tls", "key")
		if err != nil {
			return nil, err
		}
	}

	// Delete server certificate and key paths
	configValues, err = utils.DeleteFromValuesMap(configValues, "server", "https", "tls", "serverCertPath")
	if err != nil {
		return nil, err
	}
	configValues, err = utils.DeleteFromValuesMap(configValues, "server", "https", "tls", "serverKeyPath")
	if err != nil {
		return nil, err
	}

	return configValues, nil
}

func getParentGardenletDeployment() (*seedmanagementv1alpha1.GardenletDeployment, error) {
	gardenletImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenlet)
	if err != nil {
		return nil, err
	}
	gardenletImage.WithOptionalTag(version.Get().GitVersion)

	return &seedmanagementv1alpha1.GardenletDeployment{
		Image: &seedmanagementv1alpha1.Image{
			Repository: gardenletImage.Repository,
			Tag:        gardenletImage.Tag,
		},
	}, nil
}

func getParentImageVectorOverwrite() (*string, error) {
	var imageVectorOverwrite *string
	if overWritePath := os.Getenv(imagevectorutils.OverrideEnv); len(overWritePath) > 0 {
		data, err := os.ReadFile(overWritePath) // #nosec: G304 -- ImageVectorOverwrite is a feature. In reality files can be read from the Pod's file system only.
		if err != nil {
			return nil, err
		}
		imageVectorOverwrite = ptr.To(string(data))
	}
	return imageVectorOverwrite, nil
}

func getParentComponentImageVectorOverwrites() (*string, error) {
	var componentImageVectorOverwrites *string
	if overWritePath := os.Getenv(imagevectorutils.ComponentOverrideEnv); len(overWritePath) > 0 {
		data, err := os.ReadFile(overWritePath) // #nosec: G304 -- ImageVectorOverwrite is a feature. In reality files can be read from the Pod's file system only.
		if err != nil {
			return nil, err
		}
		componentImageVectorOverwrites = ptr.To(string(data))
	}
	return componentImageVectorOverwrites, nil
}
