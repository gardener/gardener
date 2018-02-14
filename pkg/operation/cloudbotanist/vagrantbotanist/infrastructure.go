// Copyright 2018 The Gardener Authors.
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

package vagrantbotanist

import (
	"fmt"
	"path/filepath"

	pb "github.com/gardener/gardener/pkg/vagrantprovider"

	"github.com/gardener/gardener/pkg/client/vagrant"
	"golang.org/x/net/context"

	"github.com/gardener/gardener/pkg/operation/common"
)

// DeployInfrastructure talks to the gardener-vagrant-provider which creates the nodes.
func (b *VagrantBotanist) DeployInfrastructure() error {

	if err := b.Operation.InitializeSeedClients(); err != nil {
		return nil
	}

	chart, err := b.Operation.ChartSeedRenderer.Render(
		filepath.Join(common.ChartPath, "seed-terraformer", "charts", "vagrant-infra"),
		"shoot-cloud-config",
		"dummy",
		b.generateTerraformInfraConfig())
	if err != nil {
		return err
	}

	var cloudConfig = ""
	for fileName, chartFile := range chart.Files {
		if fileName == "vagrant-infra/templates/config.yaml" {
			cloudConfig = chartFile
		}

	}

	client, conn, err := vagrant.New(fmt.Sprintf(b.Shoot.Info.Spec.Cloud.Vagrant.Endpoint))
	if err != nil {
		return nil
	}
	defer conn.Close()
	_, err = client.Start(context.Background(), &pb.StartRequest{
		Cloudconfig: cloudConfig,
		Id:          1,
	})

	return err
}

// DestroyInfrastructure talks to the gardener-vagrant-provider which destroys the nodes.
func (b *VagrantBotanist) DestroyInfrastructure() error {
	client, conn, err := vagrant.New(fmt.Sprintf(b.Shoot.Info.Spec.Cloud.Vagrant.Endpoint))
	if err != nil {
		return nil
	}
	defer conn.Close()
	_, err = client.Delete(context.Background(), &pb.DeleteRequest{
		Id: 1,
	})
	return nil
}

// generateTerraformInfraConfig creates the Terraform variables and the Terraform config (for the infrastructure)
// and returns them (these values will be stored as a ConfigMap and a Secret in the Garden cluster.
func (b *VagrantBotanist) generateTerraformInfraConfig() map[string]interface{} {
	var (
		sshSecret                   = b.Secrets["ssh-keypair"]
		cloudConfigDownloaderSecret = b.Secrets["cloud-config-downloader"]
	)

	return map[string]interface{}{
		"sshPublicKey": string(sshSecret.Data["id_rsa.pub"]),
		// TODO Fix this
		"workerName": "vagrant",
		"cloudConfig": map[string]interface{}{
			"kubeconfig": string(cloudConfigDownloaderSecret.Data["kubeconfig"]),
		},
	}
}

// DeployBackupInfrastructure kicks off a Terraform job which creates the infrastructure resources for backup.
func (b *VagrantBotanist) DeployBackupInfrastructure() error {
	return nil
}

// DestroyBackupInfrastructure kicks off a Terraform job which destroys the infrastructure for backup.
func (b *VagrantBotanist) DestroyBackupInfrastructure() error {
	return nil
}
