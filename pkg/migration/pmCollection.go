// Package migration migrates colelction, variables, responses to hulak
// Currently it only supports postman collection (2.1) and variables
package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/xaaha/hulak/pkg/utils"
	"github.com/xaaha/hulak/pkg/yamlparser"
)

// PmCollection represents the overall Postman collection
type PmCollection struct {
	Info     Info           `json:"info"`
	Variable []KeyValuePair `json:"variable,omitempty"`
	Item     []ItemOrReq    `json:"item"`
}

// Info represents the info object in a Postman collection
type Info struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// KeyValuePair represents a generic key-value pair used in various Postman structures
type KeyValuePair struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Type     string `json:"type,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// ItemOrReq can represent either a folder (with sub-items) or a request
// This handles the recursive nature of the structure
type ItemOrReq struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Item        []ItemOrReq `json:"item,omitempty"`    // Present if it's a folder
	Request     *Request    `json:"request,omitempty"` // Present if it's a request
	Event       []Event     `json:"event,omitempty"`   // For scripts (pre-request, test)
	Response    []Response  `json:"response,omitempty"`
}

// Event represents script events like tests or pre-request scripts
type Event struct {
	Listen string `json:"listen"`
	Script Script `json:"script"`
}

// Script contains the script content and type
type Script struct {
	Exec     []string       `json:"exec"`
	Type     string         `json:"type"`
	Packages map[string]any `json:"packages,omitempty"`
}

// Response represents saved responses
type Response struct {
	Name            string         `json:"name"`
	OriginalRequest Request        `json:"originalRequest"`
	Status          string         `json:"status"`
	Code            int            `json:"code"`
	Header          []KeyValuePair `json:"header"`
	Cookie          []any          `json:"cookie"`
	Body            string         `json:"body"`
}

// Request represents a Postman request
type Request struct {
	Method                  yamlparser.HTTPMethodType `json:"method"`
	Header                  []KeyValuePair            `json:"header"`
	Body                    *Body                     `json:"body,omitempty"`
	URL                     *PMURL                    `json:"url"`
	Description             string                    `json:"description,omitempty"`
	ProtocolProfileBehavior *map[string]any           `json:"protocolProfileBehavior,omitempty"`
}

// PMURL represents PMURL information in a request
type PMURL struct {
	Raw   yamlparser.URL `json:"raw"`
	Host  []string       `json:"host,omitempty"`
	Path  []string       `json:"path,omitempty"`
	Query []KeyValuePair `json:"query,omitempty"`
}

type graphQl struct {
	Variables string `json:"variables,omitempty" yaml:"variables"`
	Query     string `json:"query,omitempty"     yaml:"query"`
}

// Body represents request body with different modes
type Body struct {
	Mode       string         `json:"mode"`
	Raw        string         `json:"raw,omitempty"`
	URLEncoded []KeyValuePair `json:"urlencoded,omitempty"`
	FormData   []KeyValuePair `json:"formdata,omitempty"`
	GraphQL    *graphQl       `json:"graphql,omitempty"`
	Options    *BodyOptions   `json:"options,omitempty"`
}

// BodyOptions represents options for different body modes
type BodyOptions struct {
	Raw *RawOptions `json:"raw,omitempty"`
}

// RawOptions represents options specific to raw body mode
type RawOptions struct {
	Language string `json:"language"`
}

// isCollection determines if the JSON contains a Postman collection
func isCollection(jsonString map[string]any) bool {
	_, infoExists := jsonString["info"]
	_, itemExists := jsonString["item"]

	return infoExists && itemExists
}

// prepars the collection variables array to be compatible with the environment migration script
func prepareVarStr(collectionVars PmCollection) Environment {
	result := Environment{
		Name:   "",
		Scope:  "globals",
		Values: make([]EnvValues, 0, len(collectionVars.Variable)),
	}

	for _, eachItem := range collectionVars.Variable {
		envValue := EnvValues{
			Key:     eachItem.Key,
			Value:   eachItem.Value,
			Enabled: !eachItem.Disabled,
		}
		result.Values = append(result.Values, envValue)
	}

	return result
}

// saveResponses saves response examples for a single request item and returns array of JSON strings
func saveResponses(item ItemOrReq) []string {
	var responses []string

	// If no responses, return empty array
	if len(item.Response) == 0 {
		return responses
	}

	for _, response := range item.Response {
		// Create a response object
		responseData := make(map[string]any)
		responseData["name"] = item.Name
		responseData["status"] = response.Status
		responseData["code"] = response.Code

		// Add headers
		headers := make(map[string]string)
		for _, header := range response.Header {
			headers[header.Key] = header.Value
		}

		if len(headers) > 0 {
			responseData["headers"] = headers
		}

		// Parse body if it's JSON
		var bodyData any
		if err := json.Unmarshal([]byte(response.Body), &bodyData); err == nil {
			responseData["body"] = bodyData
		} else {
			responseData["body"] = response.Body
		}

		// Add request information
		requestData := make(map[string]any)
		requestData["method"] = string(response.OriginalRequest.Method)
		requestData["url"] = string(response.OriginalRequest.URL.Raw)
		responseData["request"] = requestData

		jsonBytes, err := json.MarshalIndent(responseData, "", "  ")
		if err != nil {
			continue
		}

		responses = append(responses, string(jsonBytes))
	}

	return responses
}

// converts method present in pm json file to yaml string
func methodToYaml(method yamlparser.HTTPMethodType) (string, error) {
	type YAMLOutput struct {
		Method string `yaml:"method"`
	}

	output := YAMLOutput{
		Method: string(method),
	}

	yamlBytes, err := yaml.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal method to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

func urlToYaml(pmURL PMURL) (string, error) {
	type YAMLOutput struct {
		URL       string            `yaml:"url"`
		URLParams map[string]string `yaml:"urlparams,omitempty"`
	}

	output := YAMLOutput{
		URLParams: make(map[string]string),
	}

	rawURL := string(pmURL.Raw)
	baseURL := rawURL

	if strings.Contains(rawURL, "?") {
		baseURL = strings.Split(rawURL, "?")[0]
	}

	baseURL = addDotToTemplate(baseURL)
	output.URL = baseURL

	// Process query parameters
	for _, param := range pmURL.Query {
		output.URLParams[addDotToTemplate(param.Key)] = addDotToTemplate(param.Value)
	}

	// If no URL params, remove the field
	if len(output.URLParams) == 0 {
		output.URLParams = nil
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// convet pm header from json to yaml for hulak
func headerToYAML(header []KeyValuePair) (string, error) {
	if len(header) == 0 {
		return "", nil
	}

	type HeaderOutput struct {
		Headers map[string]string `yaml:"headers"`
	}

	output := HeaderOutput{
		Headers: make(map[string]string),
	}

	for _, h := range header {
		key := addDotToTemplate(h.Key)
		value := addDotToTemplate(h.Value)

		output.Headers[key] = value
	}

	yamlBytes, err := yaml.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal headers to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// bodyToYaml converts a Postman Body struct to a YAML format that matches yamlParser.Body
func bodyToYaml(pmbody Body) (string, error) {
	yamlOutput := make(map[string]any)

	switch pmbody.Mode {
	case "raw":
		if pmbody.Raw != "" {
			yamlOutput["raw"] = addDotToTemplate(pmbody.Raw)
		}

	case "urlencoded":
		if len(pmbody.URLEncoded) > 0 {
			urlEncodedMap := make(map[string]string)
			for _, pair := range pmbody.URLEncoded {
				urlEncodedMap[addDotToTemplate(pair.Key)] = addDotToTemplate(pair.Value)
			}

			yamlOutput["urlencodedformdata"] = urlEncodedMap
		}

	case "formdata":
		if len(pmbody.FormData) > 0 {
			formDataMap := make(map[string]string)
			for _, pair := range pmbody.FormData {
				formDataMap[addDotToTemplate(pair.Key)] = addDotToTemplate(pair.Value)
			}

			yamlOutput["formdata"] = formDataMap
		}

	case "graphql":
		if pmbody.GraphQL != nil {
			graphql := make(map[string]any)
			graphql["query"] = addDotToTemplate(pmbody.GraphQL.Query)

			if len(pmbody.GraphQL.Variables) > 0 {
				variables := make(map[string]any)

				gqlVarmap := createMap(pmbody.GraphQL.Variables)
				for key, value := range gqlVarmap {
					if strValue, ok := value.(string); ok {
						variables[key] = addDotToTemplate(strValue)
					} else {
						variables[key] = value
					}
				}

				graphql["variables"] = variables
			}

			yamlOutput["graphql"] = graphql
		}

	case "none", "":
		return "", nil

	default:
		return "", fmt.Errorf("unsupported body mode: %s", pmbody.Mode)
	}

	finalOutput := map[string]any{"body": yamlOutput}
	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(finalOutput)
	if err != nil {
		return "", fmt.Errorf("failed to marshal body to YAML: %w", err)
	}

	return strings.TrimSpace(string(yamlBytes)), nil
}

// forEachRequest converts each postman request to hulak's yaml format
func forEachRequest(collection PmCollection, parentDirPath string) error {
	parentDirPath, err := utils.SanitizeDirPath(parentDirPath)
	if err != nil {
		return err
	}

	// first, move collection variables to global.env
	collectionVars := prepareVarStr(collection)
	if err := migrateEnv(collectionVars, collection.Info.Name); err != nil {
		utils.PrintRed("Error occurred while migrating Collection Variables")

		return err
	}

	return processItems(collection.Item, parentDirPath)
}

func processItems(items []ItemOrReq, parentDirPath string) error {
	counter := 0

	// Process each item
	for _, item := range items {
		itemDirPath := parentDirPath

		// If the item contains sub-items, create a directory for the item
		if len(item.Item) > 0 {
			itemDirPath = filepath.Join(parentDirPath, sanitizeKey(item.Name))
			if err := os.MkdirAll(itemDirPath, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory '%s': %w", itemDirPath, err)
			}
			// Recursively process sub-items
			if err := processItems(item.Item, itemDirPath); err != nil {
				return err
			}
		}

		if item.Request != nil {
			// Convert method to YAML
			methodYAML, err := methodToYaml(item.Request.Method)
			if err != nil {
				return fmt.Errorf("failed to convert method for request '%s': %w", item.Name, err)
			}

			var urlYAML string
			// Convert URL to YAML
			if item.Request.URL != nil {
				urlYAML, err = urlToYaml(*item.Request.URL)
				if err != nil {
					return fmt.Errorf("failed to convert URL for request '%s': %w", item.Name, err)
				}
			} else {
				return fmt.Errorf("URL is nil for request '%s'", item.Name)
			}

			// Convert headers to YAML
			headerYAML, err := headerToYAML(item.Request.Header)
			if err != nil {
				return fmt.Errorf("failed to convert headers for request '%s': %w", item.Name, err)
			}

			// Convert body to YAML if it exists
			var bodyYAML string

			if item.Request.Body != nil {
				var err error

				bodyYAML, err = bodyToYaml(*item.Request.Body)
				if err != nil {
					return fmt.Errorf("failed to convert body for request '%s': %w", item.Name, err)
				}
			}

			// Save response examples for this request
			responses := saveResponses(item)
			for i, response := range responses {
				// Create filename based on request name and response index
				sanitizedName := strings.ReplaceAll(strings.ToLower(item.Name), " ", "_")
				filename := fmt.Sprintf("%s_example_%d.json", sanitizedName, i+1)

				responseFilePath := filepath.Join(itemDirPath, filename)
				if err := os.WriteFile(responseFilePath, []byte(response), os.ModePerm); err != nil {
					return fmt.Errorf(
						"failed to write response file '%s': %w",
						responseFilePath,
						err,
					)
				}
			}

			// Build request YAML
			requestYAML := fmt.Sprintf("---\n# Request: %s\n", item.Name)

			if item.Description != "" {
				descriptionFilePath := filepath.Join(itemDirPath, "description.txt")
				if err := os.WriteFile(descriptionFilePath, []byte(item.Description), os.ModePerm); err != nil {
					return fmt.Errorf(
						"failed to write description file '%s': %w",
						descriptionFilePath,
						err,
					)
				}

				requestYAML += fmt.Sprintf("# Description: %s\n", item.Description)
			}

			// Remove prefixes and clean up the components
			methodYAML = strings.TrimPrefix(strings.TrimSpace(methodYAML), "method:")
			urlYAML = strings.TrimSpace(urlYAML)
			headerYAML = strings.TrimSpace(headerYAML)
			bodyYAML = strings.TrimSpace(bodyYAML)

			// Combine all parts with proper indentation
			requestYAML += fmt.Sprintf("method:%s\n", methodYAML)
			requestYAML += urlYAML + "\n"

			if headerYAML != "" {
				requestYAML += headerYAML + "\n"
			}

			if bodyYAML != "" {
				requestYAML += bodyYAML + "\n"
			}

			// Write each request YAML
			reqFileName := sanitizeKey(item.Name) + utils.YAML

			if item.Name == "" {
				counter++
				reqFileName = fmt.Sprintf("request_%v", counter) + utils.YAML
			}

			reqFilePath := filepath.Join(itemDirPath, reqFileName)

			if err = os.WriteFile(reqFilePath, []byte(requestYAML), utils.FilePer); err != nil {
				return fmt.Errorf("failed to write request file '%s': %w", reqFilePath, err)
			}
		}
	}

	return nil
} // migrateCollection migrates a Postman collection to the desired format
func migrateCollection(jsonStr map[string]any) error {
	// Convert the map[string]any to JSON bytes for unmarshaling into PmCollection
	jsonBytes, err := json.Marshal(jsonStr)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	// Parse JSON into PmCollection struct
	var collection PmCollection
	if err := json.Unmarshal(jsonBytes, &collection); err != nil {
		return fmt.Errorf("failed to parse collection structure: %w", err)
	}

	// create dir path
	primaryDir := sanitizeKey(collection.Info.Name)

	dirPath, err := utils.CreatePath(primaryDir)
	if err != nil {
		return err
	}
	// create dir
	if err = utils.CreateDir(dirPath); err != nil {
		return err
	}

	// inside the dir create a file for collection description
	if collection.Info.Description != "" {
		descFilePath := filepath.Join(dirPath, "description.md")
		if err = os.WriteFile(descFilePath, []byte(collection.Info.Description), utils.FilePer); err != nil {
			return err
		}
	}

	if err := forEachRequest(collection, dirPath); err != nil {
		return err
	}

	utils.PrintGreen("Collection Migration Successful! " + utils.CheckMark)

	return nil
}
