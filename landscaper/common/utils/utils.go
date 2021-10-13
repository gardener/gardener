package utils

import (
	"fmt"
	"os"

	landscaperconstants "github.com/gardener/landscaper/apis/deployer/container"
)

// GetLandscaperEnvironmentVariables reads landscaper-specific environment variables and returns their values
//  returns in order: landscaper operation, import configuration path, component descriptor path
func GetLandscaperEnvironmentVariables() (landscaperconstants.OperationType, string, string, error) {
	var operation string
	if operation = os.Getenv(landscaperconstants.OperationName); operation != string(landscaperconstants.OperationReconcile) && operation != string(landscaperconstants.OperationDelete) {
		return "", "", "", fmt.Errorf("environment variable %q has to be set and must either be %q or %q", landscaperconstants.OperationName, landscaperconstants.OperationReconcile, landscaperconstants.OperationDelete)
	}

	var importPath, componentDescriptorPath string

	if importPath = os.Getenv(landscaperconstants.ImportsPathName); importPath == "" {
		return "", "", "", fmt.Errorf("environment variable %q has to be set and point to the file containing the configuration for the controlplane landscaper", landscaperconstants.ImportsPathName)
	}

	if componentDescriptorPath = os.Getenv(landscaperconstants.ComponentDescriptorPathName); componentDescriptorPath == "" {
		return "", "", "", fmt.Errorf("environment variable %q has to be set and point to the file containing the component descriptor for the controlplane landscaper", landscaperconstants.ComponentDescriptorPathName)
	}

	return landscaperconstants.OperationType(operation), importPath, componentDescriptorPath, nil
}
