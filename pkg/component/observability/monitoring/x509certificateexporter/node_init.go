package x509certificateexporter

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"

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
		return fmt.Errorf("%w: %v", ErrMountPathNotAbsolute, m.Path)
	}
	if len(m.WatchKubeconfigs) == 0 && len(m.WatchCertificates) == 0 && len(m.WatchDirs) == 0 {
		return ErrNoMonitorableFiles
	}

	var (
		fps  = make([]string, len(m.WatchKubeconfigs)+len(m.WatchCertificates)+len(m.WatchDirs))
		errs = make([]error, 0)
	)

	fps = append(fps, m.WatchKubeconfigs...)
	fps = append(fps, m.WatchCertificates...)
	fps = append(fps, m.WatchDirs...)
	for _, path := range fps {
		if !filepath.IsAbs(path) {
			errs = append(errs, fmt.Errorf("%w: %q", ErrWatchedFileNotAbsolutePath, path))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w: %w", ErrMountValidationErrors, errors.Join(errs...))
	}
	return nil
}

func (wg *workerGroup) Validate() error {
	if wg.Selector == nil || wg.NameSuffix == "" {
		return noNodeSelectorOrNameForWorkerError(fmt.Sprintf("%+v", wg))
	}
	if len(wg.Mounts) == 0 {
		return ErrWorkerGroupMissingMount
	}
	errs := make([]error, 0)
	for k, v := range wg.Mounts {
		if err := v.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("%q %w: %w", k, ErrMountValidation, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
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
			args := make([]string, 0, countArgs()+1)
			for mountName, mount := range wg.Mounts {
				args = append(args, getArgsForMount(mountName, mount)...)
			}
			args = append(args, fmt.Sprintf("--listen-address=:%d", Port))
			return args
		}
		args = getArgs()
	)
	args = append(args, wg.GetCommonArgs()...)
	sort.Strings(args)
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
			return fmt.Errorf("%w: %w: %w", ErrWorkerGroupInvalid, ErrMultipleGroupsNoSelectorOrSuffix, err)
		}
	}
	if len(wgErrs) > 0 {
		return fmt.Errorf("%w: %w", ErrWorkerGroupInvalid, errors.Join(wgErrs...))
	}

	return nil
}
