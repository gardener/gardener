// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cache_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"

	. "github.com/gardener/gardener/pkg/client/kubernetes/cache"
)

var _ = Describe("Errors", func() {
	DescribeTable("#IsAPIError", func(err error, matcher gomegatypes.GomegaMatcher) {
		Expect(IsAPIError(err)).To(matcher)
	},
		Entry("API status Error", apierrors.NewBadRequest("foo"), BeTrue()),
		Entry("No match error for kind", &meta.NoKindMatchError{}, BeTrue()),
		Entry("No match error for resource", &meta.NoResourceMatchError{}, BeTrue()),
		Entry("Ambiguous kind error", &meta.AmbiguousKindError{}, BeTrue()),
		Entry("Ambiguous resource error", &meta.AmbiguousResourceError{}, BeTrue()),
		Entry("Any other error", fmt.Errorf("error happened"), BeFalse()),
	)

})
