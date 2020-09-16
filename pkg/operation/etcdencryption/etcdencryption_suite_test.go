// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcdencryption_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestEtcdEncryption(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "etcd Encryption Suite")
}
