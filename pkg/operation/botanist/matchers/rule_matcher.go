/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// this file is copy of https://github.com/kubernetes/kubernetes/blob/f247e75980061d7cf83c63c0fb1f12c7060c599f/staging/src/k8s.io/apiserver/pkg/admission/plugin/webhook/rules/rules.go
// with some modifications for the webhook matching use-case.
package matchers

import (
	"strings"

	"k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ruleMatcher determines if the Attr matches the Rule.
type ruleMatcher struct {
	rule        v1beta1.RuleWithOperations
	gvr         schema.GroupVersionResource
	namespace   string
	subresource string
}

// Matches returns if the resource matches the Rule.
func (r *ruleMatcher) Matches() bool {
	return r.scope() &&
		r.operation() &&
		r.group() &&
		r.version() &&
		r.resource()
}

func exactOrWildcard(items []string, requested string) bool {
	for _, item := range items {
		if item == "*" {
			return true
		}

		if item == requested {
			return true
		}
	}

	return false
}

var namespaceResource = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}

func (r *ruleMatcher) scope() bool {
	if r.rule.Scope == nil || *r.rule.Scope == v1beta1.AllScopes {
		return true
	}

	switch *r.rule.Scope {
	case v1beta1.NamespacedScope:
		// first make sure that we are not requesting a namespace object (namespace objects are cluster-scoped)
		return r.gvr != namespaceResource && r.namespace != metav1.NamespaceNone
	case v1beta1.ClusterScope:
		// also return true if the request is for a namespace object (namespace objects are cluster-scoped)
		return r.gvr == namespaceResource || r.namespace == metav1.NamespaceNone
	default:
		return false
	}
}

func (r *ruleMatcher) group() bool {
	return exactOrWildcard(r.rule.APIGroups, r.gvr.Group)
}

func (r *ruleMatcher) version() bool {
	return exactOrWildcard(r.rule.APIVersions, r.gvr.Version)
}

func (r *ruleMatcher) operation() bool {
	for _, op := range r.rule.Operations {
		switch op {
		case v1beta1.OperationAll, v1beta1.Create, v1beta1.Update:
			return true
		}
	}

	return false
}

func splitResource(resSub string) (res, sub string) {
	parts := strings.SplitN(resSub, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return parts[0], ""
}

func (r *ruleMatcher) resource() bool {
	opRes, opSub := r.gvr.Resource, r.subresource

	for _, res := range r.rule.Resources {
		res, sub := splitResource(res)
		resMatch := res == "*" || res == opRes
		subMatch := sub == "*" || sub == opSub

		if resMatch && subMatch {
			return true
		}
	}

	return false
}
