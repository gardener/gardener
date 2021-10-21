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

package executor

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/docker"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/varlibmount"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/Masterminds/sprig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	tplName = "execute-cloud-config"
	//go:embed templates/scripts/execute-cloud-config.tpl.sh
	tplContent string
	tpl        *template.Template

	tplNameCopyBinary = "copy-kubernetes-binary"
	//go:embed templates/scripts/copy-kubernetes-binary.tpl.sh
	tplContentCopyBinary string
	tplCopyBinary        *template.Template
)

func init() {
	var err error
	tpl, err = template.
		New(tplName).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContent)
	if err != nil {
		panic(err)
	}

	tplCopyBinary, err = template.
		New(tplNameCopyBinary).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentCopyBinary)
	if err != nil {
		panic(err)
	}
}

const (
	// AnnotationKeyChecksum is the key of an annotation on a shoot Node object whose value is the checksum
	// of the last applied cloud config user data.
	AnnotationKeyChecksum = "checksum/cloud-config-data"
	// ExecutionMaxDelaySeconds is a constant for the maximum delay in seconds for the execution of a downloaded
	// cloud-config user data. Each worker node will randomly select a value in [1,ExecutionMaxDelaySeconds) and always
	// delays the execution by this number.
	ExecutionMaxDelaySeconds = 300
	// PathExecutionDelaySeconds is the path on the shoot worker nodes at which the randomly computed delay for the
	// execution will be persisted.
	PathExecutionDelaySeconds = downloader.PathCCDDirectory + "/execution_delay_seconds"
	// PathExecutionLastDate is the path on the shoot worker nodes at which the date of the last execution will be
	// persisted.
	PathExecutionLastDate = downloader.PathCCDDirectory + "/execution_last_date"
	// PathHyperkubeDownloads is the path on the shoot worker nodes to which the binaries will be extracted from the
	// hyperkube image.
	PathHyperkubeDownloads = downloader.PathDownloadsDirectory + "/hyperkube"
	// PathLastDownloadedHyperkubeImage is the path on the shoot worker nodes at which the last downloaded hyperkube image
	// name will be stored.
	PathLastDownloadedHyperkubeImage = PathHyperkubeDownloads + "/last_downloaded_hyperkube_image"
	// PathHyperKubeImageUsedForLastCopyKubelet is the path on the shoot worker at which the last downloaded hyperkube image
	// name will be stored from which the kubelet binary was extracted.
	PathHyperKubeImageUsedForLastCopyKubelet = v1beta1constants.OperatingSystemConfigFilePathBinaries + "/hyperkube_image_used_for_last_copy_of_kubelet"
	// PathHyperKubeImageUsedForLastCopyKubectl is the path on the shoot worker at which the last downloaded hyperkube image
	// name will be stored from which the kubectl binary was extracted.
	PathHyperKubeImageUsedForLastCopyKubectl = v1beta1constants.OperatingSystemConfigFilePathBinaries + "/hyperkube_image_used_for_last_copy_of_kubectl"
)

// Script returns the executor script that applies the downloaded cloud-config user-data.
func Script(
	cloudConfigUserData []byte,
	hyperkubeImage *imagevector.Image,
	kubernetesVersion string,
	kubeletDataVolume *gardencorev1beta1.DataVolume,
	reloadConfigCommand string,
	units []string,
) (
	[]byte,
	error,
) {
	var scriptCopyKubernetesBinary bytes.Buffer
	if err := tplCopyBinary.Execute(&scriptCopyKubernetesBinary, map[string]interface{}{
		"pathBinaries":                             v1beta1constants.OperatingSystemConfigFilePathBinaries,
		"pathLastDownloadedHyperkubeImage":         PathLastDownloadedHyperkubeImage,
		"pathHyperkubeDownloads":                   PathHyperkubeDownloads,
		"pathHyperKubeImageUsedForLastCopyKubelet": PathHyperKubeImageUsedForLastCopyKubelet,
		"pathHyperKubeImageUsedForLastCopyKubectl": PathHyperKubeImageUsedForLastCopyKubectl,
	}); err != nil {
		return nil, err
	}

	values := map[string]interface{}{
		"annotationKeyChecksum":                    AnnotationKeyChecksum,
		"pathKubeletDirectory":                     kubelet.PathKubeletDirectory,
		"pathDownloadsDirectory":                   downloader.PathDownloadsDirectory,
		"pathBinaries":                             v1beta1constants.OperatingSystemConfigFilePathBinaries,
		"pathBootstrapToken":                       downloader.PathBootstrapToken,
		"pathCCDScript":                            downloader.PathCCDScript,
		"pathCCDScriptChecksum":                    downloader.PathCCDScriptChecksum,
		"pathCredentialsServer":                    downloader.PathCredentialsServer,
		"pathCredentialsCACert":                    downloader.PathCredentialsCACert,
		"pathDockerBinary":                         docker.PathBinary,
		"pathDownloadedCloudConfig":                downloader.PathDownloadedCloudConfig,
		"pathDownloadedChecksum":                   downloader.PathDownloadedCloudConfigChecksum,
		"pathExecutionDelaySeconds":                PathExecutionDelaySeconds,
		"pathExecutionLastDate":                    PathExecutionLastDate,
		"pathHyperkubeDownloads":                   PathHyperkubeDownloads,
		"pathHyperKubeImageUsedForLastCopyKubelet": PathHyperKubeImageUsedForLastCopyKubelet,
		"pathHyperKubeImageUsedForLastCopyKubectl": PathHyperKubeImageUsedForLastCopyKubectl,
		"pathKubeletKubeconfigBootstrap":           kubelet.PathKubeconfigBootstrap,
		"pathKubeletKubeconfigReal":                kubelet.PathKubeconfigReal,
		"pathLastDownloadedHyperkubeImage":         PathLastDownloadedHyperkubeImage,
		"pathScriptCopyKubernetesBinary":           kubelet.PathScriptCopyKubernetesBinary,
		"bootstrapTokenPlaceholder":                downloader.BootstrapTokenPlaceholder,
		"cloudConfigUserData":                      utils.EncodeBase64(cloudConfigUserData),
		"cloudConfigDownloaderName":                downloader.Name,
		"executionMinDelaySeconds":                 downloader.UnitRestartSeconds,
		"executionMaxDelaySeconds":                 ExecutionMaxDelaySeconds,
		"hyperkubeImage":                           hyperkubeImage.String(),
		"kubernetesVersion":                        kubernetesVersion,
		"reloadConfigCommand":                      reloadConfigCommand,
		"scriptCopyKubernetesBinary":               utils.EncodeBase64(scriptCopyKubernetesBinary.Bytes()),
		"units":                                    units,
		"unitNameCloudConfigDownloader":            downloader.UnitName,
		"unitNameDocker":                           docker.UnitName,
		"unitNameVarLibMount":                      varlibmount.UnitName,
	}

	if kubeletDataVolume != nil {
		dataVolumeConfig, err := getKubeletDataVolumeConfig(kubeletDataVolume)
		if err != nil {
			return nil, err
		}
		values["kubeletDataVolume"] = dataVolumeConfig
	}

	var ccdScript bytes.Buffer
	if err := tpl.Execute(&ccdScript, values); err != nil {
		return nil, err
	}

	return ccdScript.Bytes(), nil
}

func getKubeletDataVolumeConfig(volume *gardencorev1beta1.DataVolume) (map[string]interface{}, error) {
	size, err := resource.ParseQuantity(volume.VolumeSize)
	if err != nil {
		return nil, err
	}

	sizeInBytes, ok := size.AsInt64()
	if !ok {
		sizeInBytes, ok = size.AsDec().Unscaled()
		if !ok {
			return nil, fmt.Errorf("failed to parse kubelet data volume size %s", volume.VolumeSize)
		}
	}

	return map[string]interface{}{
		"name": volume.Name,
		"type": volume.Type,
		"size": fmt.Sprintf("%d", sizeInBytes),
	}, nil
}

// Secret returns a Kubernetes secret object containing the cloud-config user-data executor script.
func Secret(name, namespace, poolName string, script []byte) *corev1.Secret {
	data := map[string][]byte{downloader.DataKeyScript: script}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				downloader.AnnotationKeyChecksum: utils.ComputeSecretChecksum(data),
			},
			Labels: map[string]string{
				v1beta1constants.GardenRole:      v1beta1constants.GardenRoleCloudConfig,
				v1beta1constants.LabelWorkerPool: poolName,
			},
		},
		Data: data,
	}
}
