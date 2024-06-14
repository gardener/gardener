//  SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
//  SPDX-License-Identifier: Apache-2.0

package utils

import (
	_ "embed"
	"fmt"

	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	//go:embed extensions.yaml
	extensionsYAML string
	extensions     map[string]Extension
)

// Extension is the default specification for an `operator.gardener.cloud/v1alpha1.Extension` object.
type Extension struct {
	// Name is the name of the extension (without `gardener-extension-` prefix).
	Name string `json:"name" yaml:"name"`
	// Annotations are additional annotations that may apply to the extension and control behavior.
	Annotations map[string]string `json:"annotations" yaml:"annotations"`
	// ExtensionSpec is the specification of the `operator.gardener.cloud/v1alpha1.Extension` object.
	operatorv1alpha1.ExtensionSpec `json:",inline" yaml:",inline"`
}

func init() {
	extensionList := struct {
		Extensions []Extension `json:"extensions" yaml:"extensions"`
	}{}
	utilruntime.Must(yaml.Unmarshal([]byte(extensionsYAML), &extensionList))

	extensions = make(map[string]Extension, len(extensionList.Extensions))
	for _, extension := range extensionList.Extensions {
		extensions[extension.Name] = extension
	}
}

// Extensions returns a map whose keys are extension names and whose values are their default specs.
func Extensions() map[string]Extension {
	return extensions
}

// ExtensionSpecFor returns the spec for a given extension name. It also returns a bool indicating whether a default
// spec is known or not.
func ExtensionSpecFor(name string) (Extension, bool) {
	spec, ok := Extensions()[name]
	return spec, ok
}

// MergeExtensionSpecs takes a name and a spec. If a default spec for the given extension name is known, it merges it
// with the provided spec. The provided spec always overrides fields in the default spec. If a default spec is not
// known, then the provided spec will be returned.
func MergeExtensionSpecs(ext operatorv1alpha1.Extension) (*operatorv1alpha1.Extension, error) {
	defaultSpec, ok := ExtensionSpecFor(ext.Name)
	if !ok {
		return ptr.To(ext), nil
	}

	defaultSpecJSON, err := json.Marshal(defaultSpec.ExtensionSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default extension spec: %w", err)
	}

	specJSON, err := json.Marshal(ext.Spec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal extension spec: %w", err)
	}

	resultJSON, err := strategicpatch.StrategicMergePatch(defaultSpecJSON, specJSON, &operatorv1alpha1.ExtensionSpec{})
	if err != nil {
		return nil, fmt.Errorf("failed to merge extension specs: %w", err)
	}

	var resultSpec operatorv1alpha1.ExtensionSpec
	if err := json.Unmarshal(resultJSON, &resultSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal extension spec: %w", err)
	}

	result := ext.DeepCopy()
	result.SetAnnotations(utils.MergeStringMaps(defaultSpec.Annotations, result.Annotations))
	result.Spec = resultSpec

	return result, nil
}