// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package configvalidator

import (
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/apis/audit"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	auditvalidation "k8s.io/apiserver/pkg/apis/audit/validation"
)

var (
	strictDecoder  runtime.Decoder
	lenientDecoder runtime.Decoder
)

func init() {
	scheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(auditv1.AddToScheme, audit.AddToScheme)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
	strictDecoder = serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()
	lenientDecoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
}

// AdmitAuditPolicy validates the provided audit policy. If strict is true, unknown or misspelled fields in the
// audit policy are rejected instead of being silently dropped.
func AdmitAuditPolicy(auditPolicyRaw string, strict bool) (int32, error) {
	decoder := lenientDecoder
	if strict {
		decoder = strictDecoder
	}

	obj, schemaVersion, err := decoder.Decode([]byte(auditPolicyRaw), nil, nil)
	if err != nil {
		return http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided audit policy: %w", err)
	}

	auditPolicy, ok := obj.(*audit.Policy)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failed to cast to audit policy type: %v", schemaVersion)
	}

	if errList := auditvalidation.ValidatePolicy(auditPolicy); len(errList) != 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid audit policy: %v", errList)
	}

	return 0, nil
}
