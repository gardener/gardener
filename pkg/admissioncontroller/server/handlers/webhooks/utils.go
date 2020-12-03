// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package webhooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/gardener/gardener/pkg/logger"

	"github.com/sirupsen/logrus"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// maxRequestBody is the maximum size of the `AdmissionReview` body.
	// Same, fixed value from API server for general safety to reduce the odds of OOM issues while reading the body.
	// https://github.com/kubernetes/kubernetes/blob/d8eac8df28e6b50cd0f5380e23fc57daaf92972e/staging/src/k8s.io/apiserver/pkg/server/config.go#L322
	maxRequestBody = 3 * 1024 * 1024
)

func admissionResponse(allowed bool, msg string) *admissionv1beta1.AdmissionResponse {
	response := &admissionv1beta1.AdmissionResponse{
		Allowed: allowed,
	}

	if msg != "" {
		response.Result = &metav1.Status{
			Message: msg,
		}
	}

	return response
}

func errToAdmissionResponse(err error) *admissionv1beta1.AdmissionResponse {
	return admissionResponse(false, err.Error())
}

func respond(w http.ResponseWriter, response *admissionv1beta1.AdmissionResponse) {
	responseObj := admissionv1beta1.AdmissionReview{}
	if response != nil {
		responseObj.Response = response
	}

	jsonResponse, err := json.Marshal(responseObj)
	if err != nil {
		logger.Logger.Error(err)
	}
	if _, err := w.Write(jsonResponse); err != nil {
		logger.Logger.Error(err)
	}
}

// DecodeAdmissionRequest decodes the given http request into an admission request.
// An error is returned if the request exceeds the given limit.
func DecodeAdmissionRequest(r *http.Request, decoder runtime.Decoder, into *admissionv1beta1.AdmissionReview, limit int64, logger logrus.FieldLogger) error {
	// Read HTTP request body into variable.
	var (
		body              []byte
		wantedContentType = runtime.ContentTypeJSON
		// Increase limit by 1 (spare capacity) to determine if the limit was exceeded or right on the mark after reading.
		lr = &io.LimitedReader{R: r.Body, N: limit + 1}
	)

	if r.Body != nil {
		data, err := ioutil.ReadAll(lr)
		if err != nil {
			logger.Error(err)
			// Don't return actual error here since it might be part of a user response.
			return errors.New("an unexpected error occurred when reading the request body")
		}
		if lr.N <= 0 {
			return apierrors.NewRequestEntityTooLargeError(fmt.Sprintf("limit is %d", limit))
		}
		body = data
	}

	// Verify that the correct content-type header has been sent.
	if contentType := r.Header.Get("Content-Type"); contentType != wantedContentType {
		return fmt.Errorf("contentType not supported, expect %s", wantedContentType)
	}

	// Deserialize HTTP request body into admissionv1beta1.AdmissionReview object.
	if _, _, err := decoder.Decode(body, nil, into); err != nil {
		return err
	}

	// If the request field is empty then do not admit (invalid body).
	if into.Request == nil {
		return fmt.Errorf("invalid request body (missing admission request)")
	}

	return nil
}
