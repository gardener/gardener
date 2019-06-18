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

package packetbotanist

const (
	projectID = "projectID"
	sshKey    = "sshKey"
)

// DeployBackupInfrastructure kicks off a Terraform job which deploys the infrastructure resources for backup.
// It sets up the User and the Bucket to store the backups. Allocate permission to the User to access the bucket.
// As of this writing, Packet does not have an object store of its own. In the future, it may have one, and thus
// this will start to use it.
// Conversely, Gardener is working towards supporting object stores in a different provider than the one whose infra
// is being deployed. Once that is in, this would (optionally) run backups to AWS S3, Google Cloud, etc.
// See https://github.com/gardener/gardener/pull/932
func (b *PacketBotanist) DeployBackupInfrastructure() error {
	return nil
}

// DestroyBackupInfrastructure kicks off a Terraform job which destroys the infrastructure for etcd backup.
func (b *PacketBotanist) DestroyBackupInfrastructure() error {
	return nil
}
