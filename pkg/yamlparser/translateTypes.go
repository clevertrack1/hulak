package yamlparser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/xaaha/hulak/pkg/utils"
)

type actionType string

// These consts represent functions users can take in a yaml file
// except Invalid, which represents error
const (
	DotString  actionType = "DotString"
	GetValueOf actionType = "GetValueOf"
	Invalid    actionType = "Invalid"
)

type action struct {
	Type       actionType
	DotString  string
	GetValueOf []string
}

// checks whether string matches exactly "{{value}}"
// and retuns whether the string matches the delimiter criteria and the associated content
// So, the "{{ .value }}" returns "true, .value". Space is trimmed around the return string
func stringHasDelimiter(value string) (bool, string) {
	if len(value) < 4 || !strings.HasPrefix(value, "{{") || !strings.HasSuffix(value, "}}") {
		return false, ""
	}

	if strings.Count(value[:3], "{") > 2 || strings.Count(value[len(value)-3:], "}") > 2 {
		return false, ""
	}

	content := value[2 : len(value)-2]
	re := regexp.MustCompile(`^\s+$`)

	onlyHasEmptySpace := re.Match([]byte(value))
	if len(content) == 0 || onlyHasEmptySpace {
		return false, ""
	}

	content = strings.TrimSpace(content)

	return len(content) > 0, content
}

// Accepts a delimiterString and returns an Action struct to help the afterMap
// navigate to the appropriate dictionary for value replacement.
// Additionally, it removes double quotes, single quotes, dots (.),
// and backticks (`) from the action.
func delimiterLogicAndCleanup(delimiterString string) action {
	strHasDelimiter, innerStr := stringHasDelimiter(delimiterString)
	if !strHasDelimiter {
		return action{Type: Invalid}
	}

	innerStrChunks := strings.Split(innerStr, " ")

	// Check for DotString action
	if len(innerStrChunks) == 1 && strings.HasPrefix(innerStrChunks[0], ".") {
		dotStr := strings.TrimPrefix(innerStrChunks[0], ".")

		return action{Type: DotString, DotString: dotStr}
	}

	if len(innerStrChunks) == 3 && innerStrChunks[0] == "getValueOf" {
		cleanedChunks := cleanStrings(innerStrChunks[1:])

		return action{
			Type:       GetValueOf,
			GetValueOf: append([]string{innerStrChunks[0]}, cleanedChunks...),
		}
	}

	return action{Type: Invalid}
}

type EachGetValueofAction struct {
	Path     string
	KeyName  string
	FileName string
}

type EachDotStringAction struct {
	Path    string
	KeyName string
}

type Path struct {
	DotStrings  []EachDotStringAction
	GetValueOfs []EachGetValueofAction
}

// Recurses through the raw map prior to actions, beforeMap,
// and finds the key and it's path that needs type conversion.
// The resulting map helps us determine exact location to replace the values in afterMap
func findPathFromMap(
	beforeMap map[string]any,
	parentKey string,
) Path {
	cmprt := Path{}

	for bKey, bValue := range beforeMap {
		currentKey := bKey
		if parentKey != "" {
			currentKey = parentKey + " -> " + bKey
		}

		switch bTypeVal := bValue.(type) {
		case string:
			action := delimiterLogicAndCleanup(bTypeVal)
			if action.Type != Invalid {
				// since we only have two actions, we can keep it here.
				// but this could be a problem on large number of cases
				switch action.Type {
				case DotString:
					cmprt.DotStrings = append(cmprt.DotStrings, struct {
						Path    string
						KeyName string
					}{
						Path:    currentKey,
						KeyName: action.DotString,
					})
				case GetValueOf:
					cmprt.GetValueOfs = append(cmprt.GetValueOfs, struct {
						Path     string
						KeyName  string
						FileName string
					}{
						Path:     currentKey,
						KeyName:  action.GetValueOf[1],
						FileName: action.GetValueOf[2],
					})
				}
			}
		case map[string]any:
			subMap := findPathFromMap(bTypeVal, currentKey)
			cmprt.DotStrings = append(cmprt.DotStrings, subMap.DotStrings...)
			cmprt.GetValueOfs = append(cmprt.GetValueOfs, subMap.GetValueOfs...)
		case []any:
			for idx, val := range bTypeVal {
				arrayKey := fmt.Sprintf("%s[%d]", currentKey, idx)
				if mapVal, ok := val.(map[string]any); ok {
					subMap := findPathFromMap(mapVal, arrayKey)
					cmprt.DotStrings = append(cmprt.DotStrings, subMap.DotStrings...)
					cmprt.GetValueOfs = append(cmprt.GetValueOfs, subMap.GetValueOfs...)
				}
				// Handle other types in array if needed
			}
		case []map[string]any:
			for idx, val := range bTypeVal {
				arrayKey := fmt.Sprintf("%s[%d]", currentKey, idx)
				subMap := findPathFromMap(val, arrayKey)
				cmprt.DotStrings = append(cmprt.DotStrings, subMap.DotStrings...)
				cmprt.GetValueOfs = append(cmprt.GetValueOfs, subMap.GetValueOfs...)
			}
		default:
			// No action needed for now. We should keep expanding cases above
			// as they appear
		}
	}

	return cmprt
}

// since path gurantees that last item exists on map,
// setValueOnAfterMap walks the path and replaces the value at the last index
func setValueOnAfterMap(
	path []any,
	afterMap map[string]any,
	replaceWith any,
) map[string]any {
	var current any = afterMap

	for i, value := range path {
		switch val := value.(type) {
		case string:
			// If this is the last element in the path, set the value
			if i == len(path)-1 {
				currentMap := current.(map[string]any)
				if val, ok := currentMap[val].(string); ok {
					replaceWith = swapValue(val, replaceWith)
				}

				currentMap[val] = replaceWith
			} else {
				// Traverse deeper into the map
				current = current.(map[string]any)[val]
			}
		case int:
			// If this is the last element in the path, set the value
			if i == len(path)-1 {
				currentSlice := current.([]any)
				if val, ok := currentSlice[val].(string); ok {
					replaceWith = swapValue(val, replaceWith)
				}

				currentSlice[val] = replaceWith
			} else {
				// Traverse deeper into the slice
				current = current.([]any)[val]
			}
		}
	}

	return afterMap
}

// translateType is the function that performs translation on the `afterMap`
// based on the given `beforeMap`, `secretsMap`, and `getValueOfInterface`.
func translateType(
	beforeMap, afterMap, secretsMap map[string]any,
	getValueOf func(key, fileName string) any,
) (map[string]any, error) {
	pathMap := findPathFromMap(beforeMap, "")

	// Process dot strings
	for _, dotStringActionObj := range pathMap.DotStrings {
		path, err := parsePath(dotStringActionObj.Path)
		if err != nil {
			return nil, utils.ColorError("#TranslateType ", err)
		}

		if len(path) == 0 {
			continue
		}

		secretVal, exists := secretsMap[dotStringActionObj.KeyName]
		if !exists {
			continue
		}

		afterMap = setValueOnAfterMap(path, afterMap, secretVal)
	}

	// Process getValueOf actions
	for _, getValueOfActionObj := range pathMap.GetValueOfs {
		path, err := parsePath(getValueOfActionObj.Path)
		if err != nil {
			return nil, utils.ColorError("#TranslateType ", err)
		}

		if len(path) == 0 {
			continue
		}

		compareVal := getValueOf(getValueOfActionObj.KeyName, getValueOfActionObj.FileName)
		afterMap = setValueOnAfterMap(path, afterMap, compareVal)
	}

	return afterMap, nil
}

// Swaps val1 with val2 if the string representation of val2 is equal to val1;
// otherwise, it returns val1.
func swapValue(val1 string, val2 any) any {
	str := fmt.Sprintf("%v", val2)
	if val1 == str {
		return val2
	}

	return val1
}

// Helper function to clean strings of backtick (`), double qoutes(""), and single qoutes (”)
// around the string
func cleanStrings(stringsToClean []string) []string {
	cleaned := make([]string, len(stringsToClean))
	for i, str := range stringsToClean {
		cleaned[i] = strings.NewReplacer(`"`, "", "`", "", "'", "").Replace(str)
	}

	return cleaned
}

// Helper function for the replace in place. Parses the string with -> and array indexed strings
// Returns an array of []any ["key1", "value", 0]
func parsePath(path string) ([]any, error) {
	var keys []any

	if len(path) == 0 {
		return keys, utils.ColorError("path should not be empty")
	}

	rawKeys := strings.Split(path, "->")
	for i, segment := range rawKeys {
		trimmedKey := strings.TrimSpace(segment)
		if trimmedKey == "" {
			msg := fmt.Sprintf("Invalid format: empty key at position %d", i+1)

			return nil, utils.ColorError(msg)
		}

		isArrayKey, keyPart, index := utils.ParseArrayKey(trimmedKey)
		if isArrayKey {
			keys = append(keys, keyPart)
			keys = append(keys, index)
		} else {
			keys = append(keys, trimmedKey)
		}
	}

	return keys, nil
}
