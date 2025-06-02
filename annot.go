package main

// Returns the first annotation among the given annotation keys. When the key
// isn't found, the returned `key` is left empty.
//
// This exists because the project started with annotations starting with
// cert-manager.io/*, which caused issues:
// https://github.com/maelvls/secret-transform/issues/11.
func getOneOf(annots map[string]string, keys ...string) (key, value string) {
	if annots == nil {
		return "", ""
	}

	for _, k := range keys {
		value, found := annots[k]
		if found && value != "" {
			return k, value
		}
	}

	return "", ""
}
