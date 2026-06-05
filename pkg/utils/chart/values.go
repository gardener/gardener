// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package chart

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	apisutils "github.com/gardener/gardener/pkg/apis/utils"
	"github.com/gardener/gardener/pkg/utils"
)

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

// SubstituteTemplateInValues walks the given values structure and renders any string (both map keys and
// values) as Go templates with the provided resource references as template data. Non-string scalars are
// left untouched. The resulting structure has the same shape as the input. Templates can reference
// resolved data via `{{ .resources.<name>.data.<key> }}`.
func SubstituteTemplateInValues(values map[string]any, resources Resources) (map[string]any, error) {
	out, err := apisutils.WalkStructure(values, func(s string) (any, error) {
		return renderTemplate(s, templateData{Resources: resources})
	})
	if err != nil {
		return nil, err
	}
	result, ok := out.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected map after template substitution, got %T", out)
	}
	return result, nil
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
// expressions like `{{ .resources.<name>.data.<key> }}`. The values are walked structurally, so only references
// that appear as a complete template expression inside an individual string (map key or value) are recognized.
func ResourceNamesFromValues(values *apiextensionsv1.JSON) (sets.Set[string], error) {
	result := sets.New[string]()
	if values == nil || len(values.Raw) == 0 {
		return result, nil
	}

	var parsed any
	if err := json.Unmarshal(values.Raw, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse values: %w", err)
	}

	nameIndex := v1beta1constants.ResourceReferenceRegexp.SubexpIndex("name")
	if _, err := apisutils.WalkStructure(parsed, func(s string) (any, error) {
		for _, match := range v1beta1constants.ResourceReferenceRegexp.FindAllStringSubmatch(s, -1) {
			result.Insert(match[nameIndex])
		}
		return s, nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}
