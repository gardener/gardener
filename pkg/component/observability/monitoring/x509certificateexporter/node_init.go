package x509certificateexporter

import (
	"errors"
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func getMountedArgs(arg, mount string, filenames []string) []string {
	return mapStrings(filenames, func(value string) string {
		return fmt.Sprintf("--%s=%s/%s", mount, arg, value)
	})
}

func getCertificateFileAsArg(mount string, filenames []string) []string {
	return getMountedArgs("watch-file", mount, filenames)
}

func getKubeconfigFileAsArg(mount string, filenames []string) []string {
	return getMountedArgs("kubeconfig", mount, filenames)
}

func getCertificateDirAsArg(mount string, directories []string) []string {
	return getMountedArgs("watch-dir", mount, directories)
}

func getTrimComponentsArg(trim *uint32) []string {
	if trim != nil {
		return []string{fmt.Sprintf("--trim-path-components=%d", trim)}
	}
	return []string{}
}

// getPathSetup creates the volume and volume mount for a given path
func getPathSetup(mountPath, mountName string) (corev1.Volume, corev1.VolumeMount) {
	return corev1.Volume{
			Name: mountName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: mountPath,
					Type: ptr.To(corev1.HostPathDirectory),
				},
			},
		}, corev1.VolumeMount{
			Name:              mountName,
			ReadOnly:          true,
			RecursiveReadOnly: ptr.To(corev1.RecursiveReadOnlyIfPossible),
			MountPath:         mountPath,
		}
}

type noNodeSelectorOrNameForWorkerError string

func (n noNodeSelectorOrNameForWorkerError) Error() string {
	return "worker group " + string(n) + " must have node selector and name suffix"
}

func (m *monitorableMount) Validate() error {
	if !filepath.IsAbs(m.Path) {
		return fmt.Errorf("mount path %q is not an absolute path", m.Path)
	}
	if len(m.WatchKubeconfigs) == 0 && len(m.WatchCertificates) == 0 && len(m.WatchDirs) == 0 {
		return errors.New("at least one of watchKubeconfigs, watchCertificates, or watchDirs must be specified")
	}

	var (
		fps  = make([]string, len(m.WatchKubeconfigs)+len(m.WatchCertificates)+len(m.WatchDirs))
		errs = make([]error, 0)
	)

	fps = append(fps, m.WatchKubeconfigs...)
	fps = append(fps, m.WatchCertificates...)
	for _, path := range fps {
		if !filepath.IsAbs(path) {
			errs = append(errs, fmt.Errorf("filepath %q is not an absolute path", path))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("monitorableMount validation errors: %v", errs)
	}
	return nil
}

func (wg *workerGroup) Validate() error {
	if wg.Selector == nil || wg.NameSuffix == "" {
		return noNodeSelectorOrNameForWorkerError(fmt.Sprintf("%+v", wg))
	}
	if len(wg.Mounts) == 0 {
		return fmt.Errorf("worker group %+v must have at least one mount defined", wg)
	}
	errs := make([]error, 0)
	for k, v := range wg.Mounts {
		if err := v.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("mount %q validation error: %w", k, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("worker group %+v validation errors: %+v", wg, errs)
	}
	return nil
}

func (wg *workerGroup) GetArgs() []string {
	var (
		countArgsForMount = func(mount monitorableMount) int {
			return len(mount.WatchCertificates) + len(mount.WatchKubeconfigs) + len(mount.WatchDirs)
		}
		countArgs = func() (cnt int) {
			for _, mount := range wg.Mounts {
				cnt = cnt + countArgsForMount(mount)
			}
			return
		}
		getArgsForMount = func(mountName string, mount monitorableMount) []string {
			mountArgs := make([]string, 0, countArgsForMount(mount))
			mountArgs = append(mountArgs, getCertificateFileAsArg(mountName, mount.WatchCertificates)...)
			mountArgs = append(mountArgs, getCertificateDirAsArg(mountName, mount.WatchKubeconfigs)...)
			return append(mountArgs, getKubeconfigFileAsArg(mountName, mount.WatchDirs)...)
		}
		getArgs = func() []string {
			args := make([]string, 0, countArgs())
			for mountName, mount := range wg.Mounts {
				args = append(args, getArgsForMount(mountName, mount)...)
			}
			return args
		}
		args = getArgs()
	)
	args = append(args, getExposeRelativeMetricsArg(wg.ExposeRelativeMetrics)...)
	args = append(args, getTrimComponentsArg(wg.TrimComponents)...)
	args = append(args, getExposePerCertErrorMetricsArg(wg.ExposePerCertErrorMetrics)...)
	args = append(args, getExposeLabelsMetricsArg(wg.ExposeLabelsMetrics)...)

	return args
}

func (wgs *workerGroupsConfig) Validate() error {
	var (
		wgErrs            = make([]error, 0, len(*wgs))
		noNameOrSuffixErr noNodeSelectorOrNameForWorkerError
	)
	if len(*wgs) == 0 {
		return nil
	}
	for _, wg := range *wgs {
		err := wg.Validate()
		wgErrs = append(wgErrs, err)
		if errors.As(err, &noNameOrSuffixErr) && len(*wgs) > 1 {
			return fmt.Errorf("multiple worker groups defined, but at least one is missing a node selector: %w", err)
		}
	}
	if len(wgErrs) > 0 {
		return fmt.Errorf("workerGroups validation errors: %+v", wgErrs)
	}

	return nil
}
