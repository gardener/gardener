// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package backuprestore_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBackupRestore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Etcd Bootstrap BackupRestore Suite")
}
