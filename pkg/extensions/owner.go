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

package extensions

import (
	"context"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetOwnerNameAndID reads the owner domain name and ID from the owner DNSRecord extension resource in the given namespace.
// If the owner DNSRecord resource is not found, it returns empty strings.
func GetOwnerNameAndID(ctx context.Context, c client.Client, namespace, shootName string) (string, string, error) {
	dns := &extensionsv1alpha1.DNSRecord{}
	if err := c.Get(ctx, kutil.Key(namespace, shootName+"-owner"), dns); client.IgnoreNotFound(err) != nil && !meta.IsNoMatchError(err) {
		return "", "", err
	}

	var value string
	if len(dns.Spec.Values) > 0 {
		value = dns.Spec.Values[0]
	}

	return dns.Spec.Name, value, nil
}
