// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package chart

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"regexp"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

// resourceNameRegex searches for resources referenced via `{{ .resources.<name>.data.<key> }}`.
var resourceNameRegex = regexp.MustCompile(`\{\{-?\s*\.resources\.(?P<name>[^.\s}]+)\.data\.[^.\s}]+\s*-?\}\}`)

// Resources holds the resolved data for a list of NamedResourceReferences, keyed by reference name.
type Resources map[string]ResourceData

// ResourceData holds the resolved data of a single resource reference (a Secret or ConfigMap).
type ResourceData struct {
	// Data contains the Data entries of the referenced resource as plain strings.
	Data map[string]string `json:"data,omitempty"`
}

// templateData is the top-level template context exposed to Helm value substitution.
// Templates can reference resolved data via `{{ .resources.<name>.data.<key> }}`.
type templateData struct {
	Resources Resources `json:"resources"`
}

// ResolveResources fetches the referenced Secrets and ConfigMaps from the source namespace.
// Only references to Secrets and ConfigMaps in the core/v1 API group are supported.
func ResolveResources(ctx context.Context, c client.Client, sourceNamespace string, refs []gardencorev1.NamedResourceReference) (Resources, error) {
	result := make(Resources, len(refs))

	for _, ref := range refs {
		if ref.ResourceRef.APIVersion != "v1" {
			return nil, fmt.Errorf("resource reference %q has unsupported apiVersion %q (only v1 is supported)", ref.Name, ref.ResourceRef.APIVersion)
		}

		data := map[string]string{}
		key := types.NamespacedName{Namespace: sourceNamespace, Name: ref.ResourceRef.Name}

		switch ref.ResourceRef.Kind {
		case "Secret":
			secret := &corev1.Secret{}
			if err := c.Get(ctx, key, secret); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("referenced Secret %q for resource reference %q not found in namespace %q", ref.ResourceRef.Name, ref.Name, sourceNamespace)
				}
				return nil, fmt.Errorf("failed to get Secret %q for resource reference %q: %w", ref.ResourceRef.Name, ref.Name, err)
			}

			if secret.Labels[v1beta1constants.GardenRole] != v1beta1constants.GardenRoleResourceReference {
				return nil, fmt.Errorf("referenced Secret %q for resource reference %q does not have the label \"%s: %s\"", ref.ResourceRef.Name, ref.Name, v1beta1constants.GardenRole, v1beta1constants.GardenRoleResourceReference)
			}

			for k, v := range secret.Data {
				data[k] = string(v)
			}
		case "ConfigMap":
			configMap := &corev1.ConfigMap{}
			if err := c.Get(ctx, key, configMap); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("referenced ConfigMap %q for resource reference %q not found in namespace %q", ref.ResourceRef.Name, ref.Name, sourceNamespace)
				}
				return nil, fmt.Errorf("failed to get ConfigMap %q for resource reference %q: %w", ref.ResourceRef.Name, ref.Name, err)
			}

			if configMap.Labels[v1beta1constants.GardenRole] != v1beta1constants.GardenRoleResourceReference {
				return nil, fmt.Errorf("referenced ConfigMap %q for resource reference %q does not have the label \"%s: %s\"", ref.ResourceRef.Name, ref.Name, v1beta1constants.GardenRole, v1beta1constants.GardenRoleResourceReference)
			}

			maps.Copy(data, configMap.Data)
		default:
			return nil, fmt.Errorf("resource reference %q has unsupported kind %q (only Secret and ConfigMap are supported)", ref.Name, ref.ResourceRef.Kind)
		}

		result[ref.Name] = ResourceData{Data: data}
	}

	return result, nil
}

// SubstituteTemplateInValues walks the given values structure and renders any string values as Go templates with the
// provided resource references as template data. Non-string scalars and keys are left untouched. The resulting
// structure has the same shape as the input. Templates can reference resolved data via
// `{{ .resources.<name>.data.<key> }}`.
func SubstituteTemplateInValues(values map[string]any, resources Resources) (map[string]any, error) {
	out, err := substituteAny(values, templateData{Resources: resources})
	if err != nil {
		return nil, err
	}
	result, ok := out.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected map after template substitution, got %T", out)
	}
	return result, nil
}

func substituteAny(in any, data templateData) (any, error) {
	switch v := in.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, val := range v {
			res, err := substituteAny(val, data)
			if err != nil {
				return nil, err
			}
			out[k] = res
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, val := range v {
			res, err := substituteAny(val, data)
			if err != nil {
				return nil, err
			}
			out[i] = res
		}
		return out, nil
	case string:
		return renderTemplate(v, data)
	default:
		return v, nil
	}
}

func renderTemplate(s string, data templateData) (string, error) {
	tmpl, err := template.New("values").Option("missingkey=error").Parse(s)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %q: %w", s, err)
	}

	dataMap, err := utils.ToValuesMap(data)
	if err != nil {
		return "", fmt.Errorf("failed to convert template data: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, dataMap); err != nil {
		return "", fmt.Errorf("failed to execute template %q: %w", s, err)
	}
	return buf.String(), nil
}

// ResourceNamesFromValues returns the names of all resources referenced in the given values via template
// expressions like {{ .resources.<name>.data.<key> }}.
func ResourceNamesFromValues(values *apiextensionsv1.JSON) sets.Set[string] {
	result := sets.New[string]()
	if values == nil {
		return result
	}
	nameIndex := resourceNameRegex.SubexpIndex("name")
	for _, match := range resourceNameRegex.FindAllSubmatch(values.Raw, -1) {
		result.Insert(string(match[nameIndex]))
	}
	return result
}
