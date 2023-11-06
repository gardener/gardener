package v1alpha1

func setDefaultServerSpec(spec *KubeAPIServerOpenIDConnect) {
	if len(spec.SigningAlgs) == 0 {
		spec.SigningAlgs = []string{DefaultSignAlg}
	}

	if spec.UsernameClaim == nil {
		usernameClaim := DefaultUsernameClaim
		spec.UsernameClaim = &usernameClaim
	}
}
