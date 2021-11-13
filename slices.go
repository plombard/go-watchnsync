package fonctions

func contains(stringSlice []string, element string) bool {
	for _, value := range stringSlice {
		if value == element {
			return true
		}
	}
	return false
}

func remove(stringSlice []string, element string) []string {
	var result []string
	for _, value := range stringSlice {
		if value != element {
			result = append(result, value)
		}
	}
	return result
}

func prune(stringSlice []string) []string {
	var result []string
	for _, value := range stringSlice {
		if !contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}
