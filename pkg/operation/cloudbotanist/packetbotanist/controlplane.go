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

import (
	"github.com/gardener/etcd-backup-restore/pkg/snapstore"
)

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *PacketBotanist) GenerateEtcdBackupConfig() (map[string][]byte, error) {
	return map[string][]byte{}, nil
}

// GetEtcdBackupSnapstore returns the etcd backup snapstore object.
func (b *PacketBotanist) GetEtcdBackupSnapstore(secretData map[string][]byte) (snapstore.SnapStore, error) {
	return nil, nil
}
