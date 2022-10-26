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

package healthz

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

// NewAPIServerHealthz returns a new healthz.Checker that will pass only if the /healthz endpoint of the API server
// returns status code 200.
func NewAPIServerHealthz(ctx context.Context, restClient rest.Interface) healthz.Checker {
	return func(_ *http.Request) error {
		result := restClient.Get().AbsPath("/healthz").Do(ctx)
		if err := result.Error(); err != nil {
			return err
		}

		var statusCode int
		result.StatusCode(&statusCode)
		if statusCode != http.StatusOK {
			return fmt.Errorf("failed talking to the source cluster's kube-apiserver (status code: %d)", statusCode)
		}
		return nil
	}
}
