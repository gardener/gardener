// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package localbotanist

import (
	"fmt"

	"path/filepath"

	"github.com/gardener/gardener/pkg/client/local"
	pb "github.com/gardener/gardener/pkg/localprovider"
	"github.com/gardener/gardener/pkg/operation/common"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployInfrastructure talks to the gardener-local-provider which creates the nodes.
func (b *LocalBotanist) DeployInfrastructure() error {

	// TODO: use b.Operation.ComputeDownloaderCloudConfig("local")
	// At this stage we don't have the shoot api server
	config := map[string]interface{}{
		"kubeconfig":    string(b.Operation.Secrets["cloud-config-downloader"].Data["kubeconfig"]),
		"secretName":    b.Operation.Shoot.ComputeCloudConfigSecretName("local"),
		"cloudProvider": "local",
	}
	config, err := b.ImageVector.InjectImages(config, b.ShootVersion(), b.ShootVersion(), common.HyperkubeImageName)
	if err != nil {
		return err
	}
	chart, err := b.Operation.ChartSeedRenderer.Render(filepath.Join(common.ChartPath, "shoot-cloud-config", "charts", "downloader"), "shoot-cloud-config-downloader", metav1.NamespaceSystem, config)
	if err != nil {
		return err
	}

	var cloudConfig = ""
	for fileName, chartFile := range chart.Files {
		if fileName == "downloader/templates/cloud-config.yaml" {
			cloudConfig = chartFile
		}
	}

	client, conn, err := local.New(fmt.Sprintf(b.Shoot.Info.Spec.Cloud.Local.Endpoint))
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = client.Start(context.Background(), &pb.StartRequest{
		Cloudconfig: cloudConfig,
		Id:          1,
	})

	return err
}

// DestroyInfrastructure talks to the gardener-local-provider which destroys the nodes.
func (b *LocalBotanist) DestroyInfrastructure() error {
	client, conn, err := local.New(fmt.Sprintf(b.Shoot.Info.Spec.Cloud.Local.Endpoint))
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = client.Delete(context.Background(), &pb.DeleteRequest{
		Id: 1,
	})
	return err
}

// DeployBackupInfrastructure kicks off a Terraform job which creates the infrastructure resources for backup.
func (b *LocalBotanist) DeployBackupInfrastructure() error {
	return nil
}

// DestroyBackupInfrastructure kicks off a Terraform job which destroys the infrastructure for backup.
func (b *LocalBotanist) DestroyBackupInfrastructure() error {
	return nil
}
