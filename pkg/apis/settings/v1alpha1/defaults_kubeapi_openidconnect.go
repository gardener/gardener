package v1alpha1

func setDefaults_KubeAPIServerOpenIDConnect(obj *KubeAPIServerOpenIDConnect) {
	if len(obj.SigningAlgs) == 0 {
		obj.SigningAlgs = []string{DefaultSignAlg}
	}

	if obj.UsernameClaim == nil {
		usernameClaim := DefaultUsernameClaim
		obj.UsernameClaim = &usernameClaim
	}
}
