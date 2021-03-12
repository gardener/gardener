// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cache

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
)

// IsAPIError checks if the given error is API related.
func IsAPIError(err error) bool {
	if _, ok := err.(apierrors.APIStatus); ok {
		return true
	}
	if meta.IsNoMatchError(err) || meta.IsAmbiguousError(err) {
		return true
	}
	return false
}

// CacheError is an error type indicating that an cache error occurred.
type CacheError struct {
	cause error
}

// Unwrap returns the next error in the error chain.
func (e *CacheError) Unwrap() error {
	return e.cause
}

// Error returns the error string with the underlying error.
func (e *CacheError) Error() string {
	return fmt.Errorf("an underlying cache error occurred: %w", e.cause).Error()
}

// NewCacheError returns a new instance of `CacheError`.
func NewCacheError(err error) error {
	return &CacheError{err}
}
