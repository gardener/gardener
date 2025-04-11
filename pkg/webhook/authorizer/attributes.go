/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

The utility functions in this file were copied from the kubernetes/kubernetes project
https://github.com/kubernetes/kubernetes/blob/v1.32.3/pkg/registry/authorization/util/helpers.go

Modifications Copyright 2024 SAP SE or an SAP affiliate company and Gardener contributors
*/

package authorizer

import (
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/authentication/user"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
)

// ResourceAttributesFrom combines the API object information and the user.Info from the context to build a full
// auth.AttributesRecord for resource access.
func ResourceAttributesFrom(user user.Info, in authorizationv1.ResourceAttributes) auth.AttributesRecord {
	attrs := auth.AttributesRecord{
		User:            user,
		Verb:            in.Verb,
		Namespace:       in.Namespace,
		APIGroup:        in.Group,
		APIVersion:      in.Version,
		Resource:        in.Resource,
		Subresource:     in.Subresource,
		Name:            in.Name,
		ResourceRequest: true,
	}

	if in.LabelSelector != nil {
		if len(in.LabelSelector.RawSelector) > 0 {
			labelSelector, err := labels.Parse(in.LabelSelector.RawSelector)
			if err != nil {
				attrs.LabelSelectorRequirements, attrs.LabelSelectorParsingErr = nil, err
			} else {
				requirements, _ /*selectable*/ := labelSelector.Requirements()
				attrs.LabelSelectorRequirements, attrs.LabelSelectorParsingErr = requirements, nil
			}
		}
		if len(in.LabelSelector.Requirements) > 0 {
			attrs.LabelSelectorRequirements, attrs.LabelSelectorParsingErr = labelSelectorAsSelector(in.LabelSelector.Requirements)
		}
	}

	if in.FieldSelector != nil {
		if len(in.FieldSelector.RawSelector) > 0 {
			fieldSelector, err := fields.ParseSelector(in.FieldSelector.RawSelector)
			if err != nil {
				attrs.FieldSelectorRequirements, attrs.FieldSelectorParsingErr = nil, err
			} else {
				attrs.FieldSelectorRequirements, attrs.FieldSelectorParsingErr = fieldSelector.Requirements(), nil
			}
		}
		if len(in.FieldSelector.Requirements) > 0 {
			attrs.FieldSelectorRequirements, attrs.FieldSelectorParsingErr = fieldSelectorAsSelector(in.FieldSelector.Requirements)
		}
	}

	return attrs
}

// NonResourceAttributesFrom combines the API object information and the user.Info from the context to build a full
// auth.AttributesRecord for non resource access.
func NonResourceAttributesFrom(user user.Info, in authorizationv1.NonResourceAttributes) auth.AttributesRecord {
	return auth.AttributesRecord{
		User:            user,
		ResourceRequest: false,
		Path:            in.Path,
		Verb:            in.Verb,
	}
}

// AuthorizationAttributesFrom takes a spec and returns the proper authz attributes to check it.
func AuthorizationAttributesFrom(spec authorizationv1.SubjectAccessReviewSpec) auth.AttributesRecord {
	userToCheck := &user.DefaultInfo{
		Name:   spec.User,
		Groups: spec.Groups,
		UID:    spec.UID,
		Extra:  convertToUserInfoExtra(spec.Extra),
	}

	var authorizationAttributes auth.AttributesRecord
	if spec.ResourceAttributes != nil {
		authorizationAttributes = ResourceAttributesFrom(userToCheck, *spec.ResourceAttributes)
	} else if spec.NonResourceAttributes != nil {
		authorizationAttributes = NonResourceAttributesFrom(userToCheck, *spec.NonResourceAttributes)
	}

	return authorizationAttributes
}

func convertToUserInfoExtra(extra map[string]authorizationv1.ExtraValue) map[string][]string {
	if extra == nil {
		return nil
	}
	ret := make(map[string][]string, len(extra))
	for k, v := range extra {
		ret[k] = v
	}

	return ret
}

var labelSelectorOpToSelectionOp = map[metav1.LabelSelectorOperator]selection.Operator{
	metav1.LabelSelectorOpIn:           selection.In,
	metav1.LabelSelectorOpNotIn:        selection.NotIn,
	metav1.LabelSelectorOpExists:       selection.Exists,
	metav1.LabelSelectorOpDoesNotExist: selection.DoesNotExist,
}

func labelSelectorAsSelector(requirements []metav1.LabelSelectorRequirement) (labels.Requirements, error) {
	if len(requirements) == 0 {
		return nil, nil
	}
	reqs := make([]labels.Requirement, 0, len(requirements))
	var errs []error
	for _, expr := range requirements {
		op, ok := labelSelectorOpToSelectionOp[expr.Operator]
		if !ok {
			errs = append(errs, fmt.Errorf("%q is not a valid label selector operator", expr.Operator))
			continue
		}
		values := expr.Values
		if len(values) == 0 {
			values = nil
		}
		req, err := labels.NewRequirement(expr.Key, op, values)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		reqs = append(reqs, *req)
	}

	// If this happens, it means all requirements ended up getting skipped.
	// Return nil rather than [].
	if len(reqs) == 0 {
		reqs = nil
	}

	// Return any accumulated errors along with any accumulated requirements, so recognized / valid requirements can be considered by authorization.
	// This is safe because requirements are ANDed together so dropping unknown / invalid ones results in a strictly broader authorization check.
	return labels.Requirements(reqs), utilerrors.NewAggregate(errs)
}

func fieldSelectorAsSelector(requirements []metav1.FieldSelectorRequirement) (fields.Requirements, error) {
	if len(requirements) == 0 {
		return nil, nil
	}

	reqs := make([]fields.Requirement, 0, len(requirements))
	var errs []error
	for _, expr := range requirements {
		if len(expr.Values) > 1 {
			errs = append(errs, fmt.Errorf("fieldSelectors do not yet support multiple values"))
			continue
		}

		switch expr.Operator {
		case metav1.FieldSelectorOpIn:
			if len(expr.Values) != 1 {
				errs = append(errs, fmt.Errorf("fieldSelectors in must have one value"))
				continue
			}
			// when converting to fields.Requirement, use Equals to match how parsed field selectors behave
			reqs = append(reqs, fields.Requirement{Field: expr.Key, Operator: selection.Equals, Value: expr.Values[0]})
		case metav1.FieldSelectorOpNotIn:
			if len(expr.Values) != 1 {
				errs = append(errs, fmt.Errorf("fieldSelectors not in must have one value"))
				continue
			}
			// when converting to fields.Requirement, use NotEquals to match how parsed field selectors behave
			reqs = append(reqs, fields.Requirement{Field: expr.Key, Operator: selection.NotEquals, Value: expr.Values[0]})
		case metav1.FieldSelectorOpExists, metav1.FieldSelectorOpDoesNotExist:
			errs = append(errs, fmt.Errorf("fieldSelectors do not yet support %v", expr.Operator))
			continue
		default:
			errs = append(errs, fmt.Errorf("%q is not a valid field selector operator", expr.Operator))
			continue
		}
	}

	// If this happens, it means all requirements ended up getting skipped.
	// Return nil rather than [].
	if len(reqs) == 0 {
		reqs = nil
	}

	// Return any accumulated errors along with any accumulated requirements, so recognized / valid requirements can be considered by authorization.
	// This is safe because requirements are ANDed together so dropping unknown / invalid ones results in a strictly broader authorization check.
	return fields.Requirements(reqs), utilerrors.NewAggregate(errs)
}
