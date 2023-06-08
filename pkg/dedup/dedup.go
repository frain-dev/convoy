package dedup

import (
	"encoding/json"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Idempotency interface {
	Set(input []string, ttl int) (interface{}, error)
	Check(input []string) (bool, error)
}

type DeDuper struct {
	redis   *rdb.Redis
	request *http.Request
}

func NewDeDuper(dsn string, request *http.Request) (*DeDuper, error) {
	redis, err := rdb.NewClient(dsn)
	if err != nil {
		return nil, err
	}

	i := &DeDuper{redis, request}

	return i, nil
}

func (d *DeDuper) Set(input []string, ttl int) (interface{}, error) {
	// todo(raymond): remove interface{} return value
	parts, err := d.extractDataFromRequest(input)
	if err != nil {
		return nil, err
	}

	// build the checksum from the input parts

	// write the checksum to redis with the request details (serialize the request?)

	// return an error or nil

	return parts, err
}

func (d *DeDuper) Check(input []string) (bool, error) {
	// extract data from the request
	_, err := d.extractDataFromRequest(input)
	if err != nil {
		return false, err
	}

	// build the checksum from the input parts

	// check redis to see if the key exists

	// return an error if it does and return false

	// return true if it doesn't exist

	return false, nil
}

func (d *DeDuper) extractDataFromRequest(input []string) (interface{}, error) {
	var data []interface{}

	for _, s := range input {
		parts := strings.Split(s, ".")

		switch parts[0] {
		case "request":
			d, err := d.extractFromRequest(parts[1:])
			if err != nil {
				return nil, err
			}

			data = append(data, d)
		default:
			return nil, fmt.Errorf("unsupported input format")
		}
	}

	return data, nil
}

func (d *DeDuper) extractFromRequest(parts []string) (interface{}, error) {
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid input format")
	}

	switch parts[0] {
	case "Header":
		return d.extractFromHeader(d.request, parts[1:])
	case "Body":
		contentType := d.request.Header.Get("Content-Type")
		switch {
		case strings.HasPrefix(contentType, "application/json"):
			return d.extractFromBodyJSON(d.request, parts[1:])
		case strings.HasPrefix(contentType, "multipart/form-data"):
			return d.extractFromBodyFormData(parts[1:])
		case strings.HasPrefix(contentType, "application/x-www-form-urlencoded"):
			return d.extractFromBodyURLEncoded(parts[1:])
		default:
			return nil, fmt.Errorf("unsupported request body format: %s", contentType)
		}
	case "QueryParam":
		return d.extractFromQuery(parts[1:])
	default:
		return nil, fmt.Errorf("unsupported input format")
	}
}

func (d *DeDuper) extractFromHeader(request *http.Request, parts []string) (interface{}, error) {
	if len(parts) != 1 {
		return nil, fmt.Errorf("invalid input format")
	}

	return request.Header.Get(parts[0]), nil
}

func (d *DeDuper) extractFromBodyJSON(request *http.Request, parts []string) (interface{}, error) {
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid input format")
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return nil, err
	}

	return d.extractFromJSON(jsonData, parts)
}

func (d *DeDuper) extractFromJSON(jsonData map[string]interface{}, parts []string) (interface{}, error) {
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid input format")
	}

	value, ok := jsonData[parts[0]]
	if !ok {
		return nil, fmt.Errorf("key not found in JSON data")
	}

	if len(parts) == 1 {
		return value, nil
	}

	subJSON, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value is not a JSON object")
	}

	return d.extractFromJSON(subJSON, parts[1:])
}

func (d *DeDuper) extractFromQuery(parts []string) (interface{}, error) {
	if len(parts) != 1 {
		return nil, fmt.Errorf("invalid input format")
	}

	values, ok := d.request.URL.Query()[parts[0]]
	if !ok {
		return nil, fmt.Errorf("query parameter not found")
	}

	if len(values) > 0 {
		return values[0], nil
	}

	return nil, fmt.Errorf("query parameter is empty")
}

func (d *DeDuper) extractFromBodyFormData(parts []string) (interface{}, error) {
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid input format")
	}

	err := d.request.ParseMultipartForm(32 << 20) // 32 MB
	if err != nil {
		return nil, err
	}

	formData := d.request.MultipartForm.Value

	return d.extractFromFormValue(formData, parts)
}

func (d *DeDuper) extractFromFormValue(form url.Values, parts []string) (interface{}, error) {
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid input format")
	}

	// Perform a depth-first search to find the nested field
	nestedValue, found := d.performDFS(form, parts)
	if !found {
		return nil, fmt.Errorf("nested field not found")
	}

	return nestedValue, nil
}

func (d *DeDuper) performDFS(form url.Values, parts []string) (interface{}, bool) {
	// Base case: If there are no more parts, return the current form value
	if len(parts) == 0 {
		return form.Get(""), true
	}

	// Base case: If there is only one part, return it
	if len(parts) == 1 {
		return form.Get(parts[0]), true
	}

	// Get the next part to search for
	currentPart := parts[0]

	// Check if the current part has an index specified
	part, index := d.parsePartIndex(currentPart)
	fmt.Printf("\nindex: %v, values: %v\n", index, currentPart)

	// Check if the part exists in the form
	values, found := form[part]
	if !found {
		return nil, false
	}

	// If an index is specified, make sure it's within range
	if index != -1 && (index < 0 || index >= len(values)) {
		return nil, false
	}

	// Get the next nested form based on the part and index
	nestedForm, err := url.ParseQuery(values[index])
	if err != nil {
		return nil, false
	}

	// Recurse with the remaining parts and the nested form
	return d.performDFS(nestedForm, parts[1:])
}

func (d *DeDuper) parsePartIndex(part string) (string, int) {
	// Split the part into the field name and index (if specified)
	parts := strings.SplitN(part, "[", 2)
	field := parts[0]

	// If an index is specified, parse it
	index := -1
	if len(parts) > 1 && strings.HasSuffix(parts[1], "]") {
		indexStr := strings.TrimSuffix(parts[1], "]")
		idx, err := strconv.Atoi(indexStr)
		if err == nil {
			index = idx
		}
	}

	return field, index
}

func (d *DeDuper) extractFromBodyURLEncoded(parts []string) (interface{}, error) {
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid input format")
	}

	err := d.request.ParseForm()
	if err != nil {
		return nil, err
	}

	formData := d.request.PostForm

	return d.extractFromFormValue(formData, parts)
}
