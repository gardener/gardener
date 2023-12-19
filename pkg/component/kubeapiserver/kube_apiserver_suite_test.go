// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package kubeapiserver_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKubeAPIServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component KubeAPIServer Suite")
}
