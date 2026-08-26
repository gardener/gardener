// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package clusteridentity_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestClusterIdentity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component ClusterIdentity Suite")
}
