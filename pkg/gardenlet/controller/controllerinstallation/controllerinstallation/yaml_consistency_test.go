// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/controllerinstallation"
)

var _ = Describe("Ensuring MutateObjects makes deterministic yaml serialization ", func() {
	It("serializes yaml in a deterministic way", func() {
		content := `
---
apiVersion: v1
kind: Secret
metadata:
  annotations:
    reference.resources.gardener.cloud/configmap-32c4dfab: oidc-apps-controller-imagevector-overwrite
    reference.resources.gardener.cloud/secret-795f7ca6: garden-access-extension
    reference.resources.gardener.cloud/secret-83438e60: generic-garden-kubeconfig-a1b02908
    reference.resources.gardener.cloud/secret-8d3ae69b: oidc-apps-controller
  creationTimestamp: null
  name: foo
  namespace: bar

---
`
		stableContent := `
---
apiVersion: v1
kind: Secret
metadata:
  annotations:
    reference.resources.gardener.cloud/configmap-32c4dfab: oidc-apps-controller-imagevector-overwrite
    reference.resources.gardener.cloud/secret-8d3ae69b: oidc-apps-controller
    reference.resources.gardener.cloud/secret-795f7ca6: garden-access-extension
    reference.resources.gardener.cloud/secret-83438e60: generic-garden-kubeconfig-a1b02908
  creationTimestamp: null
  name: foo
  namespace: bar

---
`
		secret := map[string][]byte{
			"keyA": []byte(content),
		}
		err := controllerinstallation.MutateObjects(secret, func(_ *unstructured.Unstructured) error {
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(secret["keyA"])).To(Equal(stableContent))
	})

})
