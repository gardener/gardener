// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectNames takes an ObjectList and returns the corresponding names of the objects.
func ObjectNames(list client.ObjectList) []string {
	GinkgoHelper()

	names := make([]string, 0, meta.LenList(list))
	err := meta.EachListItem(list, func(o runtime.Object) error {
		names = append(names, o.(client.Object).GetName())
		return nil
	})

	Expect(err).NotTo(HaveOccurred())
	return names
}
