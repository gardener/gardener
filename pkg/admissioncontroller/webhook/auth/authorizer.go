// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"

	"github.com/gardener/gardener/pkg/utils/graph"
	authorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer"
)

// CheckVerb checks if the verbs in the attributes is allowed for the resource type.
func CheckVerb(log logr.Logger, attrs auth.Attributes, allowedVerbs ...string) (bool, string) {
	if !slices.Contains(allowedVerbs, attrs.GetVerb()) {
		log.Info("Denying authorization because verb is not allowed for this resource type", "allowedVerbs", allowedVerbs)
		return false, fmt.Sprintf("only the following verbs are allowed for this resource type: %+v", allowedVerbs)
	}

	return true, ""
}

// CheckSubresource checks if the subresource in the attributes is allowed for the resource type. If no subresource is
// provided in the attributes, the check always passes.
func CheckSubresource(log logr.Logger, attrs auth.Attributes, allowedSubresources ...string) (bool, string) {
	if subresource := attrs.GetSubresource(); len(subresource) > 0 && !slices.Contains(allowedSubresources, subresource) {
		log.Info("Denying authorization because subresource is not allowed for this resource type", "allowedSubresources", allowedSubresources)
		return false, fmt.Sprintf("only the following subresources are allowed for this resource type: %+v", allowedSubresources)
	}

	return true, ""
}

type authzRequest struct {
	allowedVerbs        sets.Set[string]
	alwaysAllowedVerbs  sets.Set[string]
	allowedSubresources sets.Set[string]
	listWatchSelector   selector
}

func newAuthzRequest() *authzRequest {
	return &authzRequest{
		allowedVerbs:        sets.Set[string]{},
		alwaysAllowedVerbs:  sets.Set[string]{},
		allowedSubresources: sets.Set[string]{},
		listWatchSelector:   selector{fieldNames: sets.Set[string]{}, labelKeys: sets.Set[string]{}},
	}
}

type selector struct {
	fieldNames sets.Set[string]
	labelKeys  sets.Set[string]
}

type configFunc func(req *authzRequest)

// WithAllowedVerbs is a config function for setting the allowed verbs.
func WithAllowedVerbs(verbs ...string) configFunc {
	return func(req *authzRequest) {
		req.allowedVerbs.Insert(verbs...)
	}
}

// WithAlwaysAllowedVerbs is a config function for setting the always allowed verbs.
func WithAlwaysAllowedVerbs(verbs ...string) configFunc {
	return func(req *authzRequest) {
		req.alwaysAllowedVerbs.Insert(verbs...)
	}
}

// WithAllowedSubresources is a config function for setting the allowed subresources.
func WithAllowedSubresources(resources ...string) configFunc {
	return func(req *authzRequest) {
		req.allowedSubresources.Insert(resources...)
	}
}

// WithFieldSelectorFields is a config function for setting the field selector fields. Field names are matched against
// the name of the object the requestor is associated with. For example, if the field name is '.spec.seedName' and the
// requestor is associated with seed 'foo', then they must send a field selector for '.spec.seedName=foo' in order to be
// authorized for list/watch requests.
// TODO(rfranzke): Remove this 'nolint' annotation once the function is used.
//
//nolint:unused
func WithFieldSelectorFields(fieldNames ...string) configFunc {
	return func(req *authzRequest) {
		req.listWatchSelector.fieldNames.Insert(fieldNames...)
	}
}

// WithLabelSelectorKeys is a config function for setting the label selector keys. Label keys must contain the name of
// the object the requestor is associated with. For example, if the label key is 'name.seed.gardener.cloud/foo' and the
// requestor is associated with seed 'foo', then they must send a label selector for 'name.seed.gardener.cloud/foo=true'
// in order to be authorized for list/watch requests.
// TODO(rfranzke): Remove this 'nolint' annotation once the function is used.
//
//nolint:unused
func WithLabelSelectorKeys(labelKeys ...string) configFunc {
	return func(req *authzRequest) {
		req.listWatchSelector.labelKeys.Insert(labelKeys...)
	}
}

// RequestAuthorizer contains common fields that can be used to authorize requests based on graph relationships.
type RequestAuthorizer struct {
	Log                    logr.Logger
	Graph                  graph.Interface
	AuthorizeWithSelectors authorizerwebhook.WithSelectorsChecker

	ToType      graph.VertexType
	ToNamespace string
	ToName      string
}

// CheckRead checks if a read request (get, list, watch) is allowed based on the graph relationships and the provided
// attributes.
func (a *RequestAuthorizer) CheckRead(fromType graph.VertexType, attrs auth.Attributes) (auth.Decision, string, error) {
	return a.Check(fromType, attrs,
		WithAllowedVerbs("list", "watch", "get"),
	)
}

// Check checks if a request is allowed based on the graph relationships and the provided attributes.
func (a *RequestAuthorizer) Check(fromType graph.VertexType, attrs auth.Attributes, fns ...configFunc) (auth.Decision, string, error) {
	log := a.Log.WithValues(
		"fromType", graph.VertexTypes[fromType].Kind,
		"fromNamespace", attrs.GetNamespace(),
		"fromName", attrs.GetName(),
		"toType", graph.VertexTypes[a.ToType].Kind,
		"toNamespace", a.ToNamespace,
		"toName", a.ToName,
	)

	req := newAuthzRequest()
	for _, f := range fns {
		f(req)
	}

	if ok, reason := CheckSubresource(log, attrs, sets.List(req.allowedSubresources)...); !ok {
		return auth.DecisionNoOpinion, reason, nil
	}

	// When a new object is created then it doesn't yet exist in the graph, so usually such requests are always allowed
	// as the 'create case' is typically handled in the respective restriction admission handler. Similarly, resources
	// for which the gardenlet has a controller need to be listed/watched, so those verbs would also be allowed here.
	if req.alwaysAllowedVerbs.Has(attrs.GetVerb()) {
		return auth.DecisionAllow, "", nil
	}

	if ok, reason := CheckVerb(log, attrs, sets.List(req.alwaysAllowedVerbs.Union(req.allowedVerbs))...); !ok {
		return auth.DecisionNoOpinion, reason, nil
	}

	if (attrs.GetVerb() == "list" || attrs.GetVerb() == "watch") &&
		// A resource name is also set when a specific object is read with `.metadata.name` field selector (e.g., in the
		// single object cache), even for the list verb.
		// If we have a resource name then we want to consult the graph. There is an exception, though, which is when
		// the request specifies `.metadata.name` as field name for a selector. This means that the client wants to
		// list/watch the resource with the object name as field selector. This is a valid scenario and needs to be
		// handled by the below check function.
		(attrs.GetName() == "" || req.listWatchSelector.fieldNames.Has(metav1.ObjectNameField)) {
		if canAuthorizeWithSelectors, err := a.AuthorizeWithSelectors.IsPossible(); err != nil {
			return auth.DecisionNoOpinion, "", fmt.Errorf("failed checking if authorization with selectors is possible: %w", err)
		} else if canAuthorizeWithSelectors {
			if ok, reason := a.checkListWatchRequests(attrs, req.listWatchSelector); !ok {
				log.Info("Denying list/watch request because field/label selectors don't the 'to object'", "fieldNames", req.listWatchSelector.fieldNames, "labelKeys", req.listWatchSelector.labelKeys)
				return auth.DecisionNoOpinion, reason, nil
			} else {
				return auth.DecisionAllow, "", nil
			}
		} else if len(req.listWatchSelector.labelKeys) > 0 || len(req.listWatchSelector.fieldNames) > 0 {
			// TODO(rfranzke): Remove this else-if branch once the lowest supported Kubernetes version is 1.34.
			return auth.DecisionAllow, "", nil
		}
	}

	return a.hasPathFrom(log, fromType, attrs)
}

func (a *RequestAuthorizer) checkListWatchRequests(attrs auth.Attributes, selector selector) (bool, string) {
	// The authorization request originates from the kube-apiserver. It has already parsed the field/label selector
	// and converted it to {fields,labels}.Requirements. Hence, it is safe to ignore the error here. Furthermore, we
	// require at least one selector. When the parsing failed, the list of selectors would be empty, resulting in
	// below code to deny the request anyway.
	fieldSelectorRequirements, _ := attrs.GetFieldSelector()
	labelSelectorRequirements, _ := attrs.GetLabelSelector()

	for _, req := range fieldSelectorRequirements {
		if (req.Operator == selection.Equals || req.Operator == selection.DoubleEquals || req.Operator == selection.In) &&
			req.Value == a.ToName &&
			selector.fieldNames.Has(req.Field) {
			return true, "field selector provided and matches name of 'to object'"
		}
	}

	for _, req := range labelSelectorRequirements {
		for key := range selector.labelKeys {
			if req.Matches(labels.Set{key: "true"}) {
				return true, "label selector provided and matches  name of 'to object'"
			}
		}
	}

	return false, fmt.Sprintf("must specify field or label selector for name %s", a.ToName)
}

func (a *RequestAuthorizer) hasPathFrom(log logr.Logger, fromType graph.VertexType, attrs auth.Attributes) (auth.Decision, string, error) {
	if len(attrs.GetName()) == 0 {
		log.Info("Denying authorization because attributes are missing object name")
		return auth.DecisionNoOpinion, "No Object name found", nil
	}

	// If the request is made for a namespace then the attributes.Namespace field is not empty. It contains the name of
	// the namespace.
	namespace := attrs.GetNamespace()
	if fromType == graph.VertexTypeNamespace {
		namespace = ""
	}

	// If the vertex does not exist in the graph (i.e., the resource does not exist in the system) then we allow the
	// request.
	if attrs.GetVerb() == "delete" && !a.Graph.HasVertex(fromType, namespace, attrs.GetName()) {
		return auth.DecisionAllow, "", nil
	}

	if !a.Graph.HasPathFrom(fromType, namespace, attrs.GetName(), a.ToType, a.ToNamespace, a.ToName) {
		log.Info("Denying authorization because no relationship is found")
		return auth.DecisionNoOpinion, "no relationship found", nil
	}

	return auth.DecisionAllow, "", nil
}
