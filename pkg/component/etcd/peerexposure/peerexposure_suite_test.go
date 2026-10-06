// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package peerexposure_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPeerExposure(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Etcd PeerExposure Suite")
}
