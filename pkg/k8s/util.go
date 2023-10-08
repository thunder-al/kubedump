package k8s

func IsIncluded(name string, include, exclude []string) bool {
	for _, item := range exclude {
		if item == name {
			return false
		}
	}

	if len(include) > 0 {
		for _, item := range include {
			if item == name {
				return true
			}
		}
		return false
	}

	return true
}

func IsIncludedAny(names []string, include, exclude []string) bool {
	for _, name := range names {
		if IsIncluded(name, include, exclude) {
			return true
		}
	}

	return false
}
