package merge

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/intervention-engine/fhir/models"
)

// traverse recursively iterates through all non-nil fields in a resource, identifying the JSON paths
// to non-nil fields. Each path and the value at that path (a primitive go type, or a time.Time object)
// is collected in the PathMap for later reference or comparison. To build a path, each exported field in
// a resource must have a "json" struct tag. In practice this is true of all intervention-engine/fhir models.
func traverse(value reflect.Value, paths PathMap, path string) {
	switch value.Kind() {
	case reflect.Ptr, reflect.Interface:
		// To get the actual value of the object or interface being pointed to we use Elem().
		val := value.Elem()
		// Check if the pointer or interface is nil.
		if !val.IsValid() {
			return
		}

		// Traverse the object that's being pointed to.
		traverse(val, paths, path)

	case reflect.Struct:
		// We don't traverse into FHIRDateTime objects.
		_, ok := value.Interface().(models.FHIRDateTime)
		if ok {
			paths[path] = value
			return
		}

		// Traverse all non-nil fields in the struct, building up their json paths.
		for i := 0; i < value.NumField(); i++ {
			jsonPath := value.Type().Field(i).Tag.Get("json")
			// jsonPath will be empty for inline resources (e.g. DomainResource).
			if jsonPath != "" {
				prefix := ""
				// The path is empty if we're currently traversing the top-level object (e.g. Patient).
				if path != "" {
					prefix = path + "."
				}
				// Splits into the name of the field (e.g. "gender") and the "omitempty" flag.
				parts := strings.SplitN(jsonPath, ",", 2)
				traverse(value.Field(i), paths, prefix+parts[0])
			} else {
				// This was an inline resource, so we shouldn't add it to the path. Just traverse
				// it's fields instead.
				traverse(value.Field(i), paths, path)
			}
		}

	case reflect.Slice, reflect.Array:
		// Traverse all elements in the slice.
		for i := 0; i < value.Len(); i++ {
			traverse(value.Index(i), paths, path+fmt.Sprintf("[%d]", i))
		}

	case reflect.String:
		// Check that the string isn't nil.
		val := value.String()
		if val != "" {
			paths[path] = value
		}

	default:
		// These are all of the other primitive types (e.g. int, float, bool).
		paths[path] = value
	}
}
