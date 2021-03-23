// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"crypto/sha1"
	"encoding/base32"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	lsv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
)

const Base32EncodeStdLowerCase = "abcdefghijklmnopqrstuvwxyz234567"

const SourceDelimiter = "/"

const NonContextifiedPrefix = "#"

// InstallationPrefix is the prefix combined with installation name is used as label value. Do not change length.
const InstallationPrefix = "Inst."

// ExecutionPrefix is the prefix combined with execution name is used as label value. Do not change length.
const ExecutionPrefix = "Exec."

// GenerateDataObjectName generates the unique name for a data object exported or imported by a installation.
// It returns a non contextified data name if the name starts with a "#".
func GenerateDataObjectName(context string, name string) string {
	if strings.HasPrefix(name, NonContextifiedPrefix) {
		return strings.TrimPrefix(name, NonContextifiedPrefix)
	}
	doName := fmt.Sprintf("%s/%s", context, name)
	h := sha1.New()
	_, _ = h.Write([]byte(doName))
	// we need base32 encoding as some base64 (even url safe base64) characters are not supported by k8s
	// see https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
	return base32.NewEncoding(Base32EncodeStdLowerCase).WithPadding(base32.NoPadding).EncodeToString(h.Sum(nil))
}

// DataObjectSourceFromObject returns the data object source for a runtime object.
func DataObjectSourceFromObject(src runtime.Object) (string, error) {
	acc, ok := src.(metav1.Object)
	if !ok {
		return "", fmt.Errorf("source has to be a kubernetes metadata object")
	}

	srcKind := src.GetObjectKind().GroupVersionKind().Kind
	return srcKind + SourceDelimiter + acc.GetNamespace() + SourceDelimiter + acc.GetName(), nil
}

// ObjectFromDataObjectSource parses the source's kind, namespace and name from a src string.
func ObjectFromDataObjectSource(src string) (string, lsv1alpha1.ObjectReference, error) {
	splitValues := strings.Split(src, SourceDelimiter)
	if len(splitValues) != 3 {
		return "", lsv1alpha1.ObjectReference{}, fmt.Errorf("expected source definition with 3 paramters but got %d", len(splitValues))
	}

	kind, namespace, name := splitValues[0], splitValues[1], splitValues[2]
	return kind, lsv1alpha1.ObjectReference{Namespace: namespace, Name: name}, nil
}

// DataObjectSourceFromInstallation returns the data object source for a Installation.
func DataObjectSourceFromInstallation(src *lsv1alpha1.Installation) string {
	return InstallationPrefix + src.GetName()
}

// DataObjectSourceFromExecution returns the data object source for a Execution.
func DataObjectSourceFromExecution(src *lsv1alpha1.Execution) string {
	return ExecutionPrefix + src.GetName()
}
