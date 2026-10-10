// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// EventuallyNotHaveOperationAnnotation waits for the gardener operation annotation to be removed from the given object.
// It must only be called inside a running Ginkgo node (e.g., an It node), never during tree construction: the komega
// instance and object are typically fields of a test context that is only initialized in a BeforeAll node. Passing them
// to a helper during tree construction would capture their zero values instead.
func EventuallyNotHaveOperationAnnotation(ctx context.Context, komega komega.Komega, obj client.Object) {
	GinkgoHelper()

	Eventually(ctx, komega.Object(obj)).WithPolling(2 * time.Second).Should(
		HaveField("ObjectMeta.Annotations", Not(HaveKey(v1beta1constants.GardenerOperation))))
}
