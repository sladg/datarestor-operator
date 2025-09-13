package utils

func MakeAnnotation(existing map[string]string, new map[string]string) map[string]string {
	if existing == nil {
		existing = make(map[string]string)
	}

	result := make(map[string]string)
	for k, v := range existing {
		result[k] = v
	}

	for k, v := range new {
		if v == "" {
			// Empty value should remove the key if it exists
			delete(result, k)
		} else {
			result[k] = v
		}
	}

	return result
}
