// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedadmission_test

import (
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"gomodules.xyz/jsonpatch/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestSeedadmission(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Seed Admission Suite")
}

func expectAllowed(response admission.Response, reason gomegatypes.GomegaMatcher, optionalDescription ...interface{}) {
	Expect(response.Allowed).To(BeTrue(), optionalDescription...)
	Expect(string(response.Result.Reason)).To(reason, optionalDescription...)
}

func expectPatched(response admission.Response, reason gomegatypes.GomegaMatcher, patches []jsonpatch.JsonPatchOperation, optionalDescription ...interface{}) {
	expectAllowed(response, reason, optionalDescription...)
	Expect(response.Patches).To(Equal(patches))
}

func expectDenied(response admission.Response, reason gomegatypes.GomegaMatcher, optionalDescription ...interface{}) {
	Expect(response.Allowed).To(BeFalse(), optionalDescription...)
	Expect(response.Result.Code).To(BeEquivalentTo(http.StatusForbidden), optionalDescription...)
	Expect(string(response.Result.Reason)).To(reason, optionalDescription...)
}

func expectErrored(response admission.Response, code, err gomegatypes.GomegaMatcher, optionalDescription ...interface{}) {
	Expect(response.Allowed).To(BeFalse(), optionalDescription...)
	Expect(response.Result.Code).To(code, optionalDescription...)
	Expect(response.Result.Message).To(err, optionalDescription...)
}
