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

func removeIntersec(sliceA, sliceB []string) (resultA []string, resultB []string) {
	resultB = append(sliceB)
	for _, value := range sliceA {
		if contains(sliceB, value) {
			resultB = remove(resultB, value)
		} else {
			resultA = append(resultA, value)
		}
	}
	return resultA, resultB
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
