// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import "k8s.io/apimachinery/pkg/api/resource"

// BoolPtr returns a bool pointer to its argument.
func BoolPtr(b bool) *bool {
	return &b
}

// Int32Ptr returns a int32 pointer to its argument.
func Int32Ptr(i int32) *int32 {
	return &i
}

// StringPtr returns a String pointer to its argument.
func StringPtr(s string) *string {
	return &s
}

// QuantityPtr returns a Quatity pointer to its argument
func QuantityPtr(q resource.Quantity) *resource.Quantity {
	return &q
}
