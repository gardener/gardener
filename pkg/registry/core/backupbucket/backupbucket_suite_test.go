// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBackupbucket(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registry BackupBucket Suite")
}
