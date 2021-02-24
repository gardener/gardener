// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pkg

func (g Landscaper) computeGardenletChartValues(bootstrapKubeconfig []byte) map[string]interface{} {
	gardenClientConnection := map[string]interface{}{
		"acceptContentTypes": g.gardenletConfiguration.GardenClientConnection.AcceptContentTypes,
		"contentType":        g.gardenletConfiguration.GardenClientConnection.ContentType,
		"qps":                g.gardenletConfiguration.GardenClientConnection.QPS,
		"burst":              g.gardenletConfiguration.GardenClientConnection.Burst,
		// always set in defaults
		"kubeconfigSecret":   *g.gardenletConfiguration.GardenClientConnection.KubeconfigSecret,
	}

	// setting a bootstrap secret is only required during the first deployment
	// after, the automatic cert rotation of the Gardenlet uses the existing client certificate
	// to rotate the credentials
	if len(bootstrapKubeconfig) > 0 {
		gardenClientConnection["bootstrapKubeconfig"] = map[string]interface{}{
			"name":       g.gardenletConfiguration.GardenClientConnection.BootstrapKubeconfig.Name,
			"namespace":  g.gardenletConfiguration.GardenClientConnection.BootstrapKubeconfig.Namespace,
			"kubeconfig": string(bootstrapKubeconfig),
		}
	}

	if g.gardenletConfiguration.GardenClientConnection.GardenClusterAddress != nil {
		gardenClientConnection["gardenClusterAddress"] = *g.gardenletConfiguration.GardenClientConnection.GardenClusterAddress
	}

	configValues := map[string]interface{}{
		"gardenClientConnection": gardenClientConnection,
		"featureGates":           g.gardenletConfiguration.FeatureGates,
		"server":                 *g.gardenletConfiguration.Server,
		"seedConfig":             *g.gardenletConfiguration.SeedConfig,
	}

	if g.gardenletConfiguration.ShootClientConnection != nil {
		configValues["shootClientConnection"] = *g.gardenletConfiguration.ShootClientConnection
	}

	if g.gardenletConfiguration.SeedClientConnection != nil {
		configValues["seedClientConnection"] = *g.gardenletConfiguration.SeedClientConnection
	}

	if g.gardenletConfiguration.Controllers != nil {
		configValues["controllers"] = *g.gardenletConfiguration.Controllers
	}

	if g.gardenletConfiguration.LeaderElection != nil {
		configValues["leaderElection"] = *g.gardenletConfiguration.LeaderElection
	}

	if g.gardenletConfiguration.LogLevel != nil {
		configValues["logLevel"] = *g.gardenletConfiguration.LogLevel
	}

	if g.gardenletConfiguration.KubernetesLogLevel != nil {
		configValues["kubernetesLogLevel"] = *g.gardenletConfiguration.KubernetesLogLevel
	}

	if g.gardenletConfiguration.Logging != nil {
		configValues["logging"] = *g.gardenletConfiguration.Logging
	}

	gardenletValues := map[string]interface{}{
		"image": map[string]interface{}{
			"repository": g.gardenletImageRepository,
			"tag":        g.gardenletImageTag,
		},
		"config": configValues,
	}

	// Set deployment values
	if g.Imports.DeploymentConfiguration.ReplicaCount != nil {
		gardenletValues["replicaCount"] = *g.Imports.DeploymentConfiguration.ReplicaCount
	}

	if g.Imports.DeploymentConfiguration.ServiceAccountName != nil {
		gardenletValues["serviceAccountName"] = *g.Imports.DeploymentConfiguration.ServiceAccountName
	}

	if g.Imports.DeploymentConfiguration.RevisionHistoryLimit != nil {
		gardenletValues["revisionHistoryLimit"] = *g.Imports.DeploymentConfiguration.RevisionHistoryLimit
	}

	if g.Imports.DeploymentConfiguration.ImageVectorOverwrite != nil {
		gardenletValues["imageVectorOverwrite"] = *g.Imports.DeploymentConfiguration.ImageVectorOverwrite
	}

	if g.Imports.DeploymentConfiguration.ComponentImageVectorOverwrites != nil {
		gardenletValues["componentImageVectorOverwrites"] = *g.Imports.DeploymentConfiguration.ComponentImageVectorOverwrites
	}

	if g.Imports.DeploymentConfiguration.Resources != nil {
		gardenletValues["resources"] = *g.Imports.DeploymentConfiguration.Resources
	}

	if g.Imports.DeploymentConfiguration.PodLabels != nil {
		gardenletValues["podLabels"] = g.Imports.DeploymentConfiguration.PodLabels
	}

	if g.Imports.DeploymentConfiguration.PodAnnotations != nil {
		gardenletValues["podAnnotations"] = g.Imports.DeploymentConfiguration.PodAnnotations
	}

	if g.Imports.DeploymentConfiguration.AdditionalVolumeMounts != nil {
		gardenletValues["additionalVolumeMounts"] = g.Imports.DeploymentConfiguration.AdditionalVolumeMounts
	}

	if g.Imports.DeploymentConfiguration.AdditionalVolumes != nil {
		gardenletValues["additionalVolumes"] = g.Imports.DeploymentConfiguration.AdditionalVolumes
	}

	if g.Imports.DeploymentConfiguration.Env != nil {
		gardenletValues["env"] = g.Imports.DeploymentConfiguration.Env
	}

	if g.Imports.DeploymentConfiguration.VPA != nil {
		gardenletValues["vpa"] = *g.Imports.DeploymentConfiguration.VPA
	}

	return map[string]interface{}{
		"global": map[string]interface{}{
			"gardenlet": gardenletValues,
		},
	}
}
