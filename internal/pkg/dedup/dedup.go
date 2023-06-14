package dedup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Idempotency interface {
	Set(source string, input []string, ttl time.Duration) error
	Exists(source string, input []string) (bool, error)
}

type DeDuper struct {
	ctx     context.Context
	cache   cache.Cache
	request *http.Request
}

func NewDeDuper(ctx context.Context, cache cache.Cache, request *http.Request) *DeDuper {
	return &DeDuper{ctx, cache, request}
}

// Set generates a checksum using the provided request input fields, creates a checksum
// and writes it to redis with a ttl.
func (d *DeDuper) Set(source string, input []string, ttl time.Duration) (string, error) {
	parts, err := d.extractDataFromRequest(input)
	if err != nil {
		return "", err
	}

	// build the checksum from the input parts
	var builder strings.Builder
	builder.WriteString(source)
	for i := range parts {
		builder.WriteString(fmt.Sprintf("%v", parts[i]))
	}

	checksum := calculateChecksum(builder.String())

	key := convoy.IdempotencyCacheKey.Get(checksum).String()
	err = d.cache.Set(d.ctx, key, true, ttl)
	if err != nil {
		return "", err
	}

	return checksum, nil
}

func (d *DeDuper) Exists(source string, input []string) (bool, error) {
	// extract data from the request
	parts, err := d.extractDataFromRequest(input)
	if err != nil {
		return false, err
	}

	// build the checksum from the input parts
	var builder strings.Builder
	builder.WriteString(source)
	for i := range parts {
		builder.WriteString(fmt.Sprintf("%v", parts[i]))
	}

	checksum := calculateChecksum(builder.String())

	key := convoy.IdempotencyCacheKey.Get(checksum).String()
	var data bool

	err = d.cache.Get(d.ctx, key, &data)
	if err != nil {
		return false, err
	}

	return data, nil
}

func (d *DeDuper) extractDataFromRequest(input []string) ([]interface{}, error) {
	var data []interface{}

	for _, s := range input {
		parts := strings.Split(s, ".")

		switch parts[0] {
		case "request", "req":
			d, err := d.extractFromRequest(parts[1:])
			if err != nil {
				return nil, err
			}

			data = append(data, d)
		default:
			return nil, fmt.Errorf("unsupported input format for idempotency key")
		}
	}

	return data, nil
}

func (d *DeDuper) extractFromRequest(parts []string) (interface{}, error) {
	if len(parts) < 2 {
		return nil, fmt.Errorf("not enough parts")
	}

	switch parts[0] {
	case "Header", "header":
		return d.extractFromHeader(d.request, parts[1:])
	case "Body", "body":
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
	case "QueryParam", "query":
		return d.extractFromQuery(parts[1:])
	default:
		return nil, fmt.Errorf("unsupported input format")
	}
}

func (d *DeDuper) extractFromHeader(request *http.Request, parts []string) (interface{}, error) {
	if len(parts) != 1 {
		return nil, fmt.Errorf("unsupported input format for idempotency key")
	}

	return request.Header.Get(parts[0]), nil
}

func (d *DeDuper) extractFromBodyJSON(request *http.Request, parts []string) (interface{}, error) {
	if len(parts) < 1 {
		return nil, fmt.Errorf("unsupported input format for idempotency key")
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}

	request.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var q strings.Builder
	q.WriteString(parts[0])
	for i := 1; i < len(parts); i++ {
		q.WriteString(fmt.Sprintf(".%v", parts[i]))
	}

	return gjson.GetBytes(body, q.String()), nil
}

func (d *DeDuper) extractFromQuery(parts []string) (interface{}, error) {
	if len(parts) != 1 {
		return nil, fmt.Errorf("unsupported input format for idempotency key")
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
		return nil, fmt.Errorf("unsupported input format for idempotency key")
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
		return nil, fmt.Errorf("unsupported input format for idempotency key")
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
		return nil, fmt.Errorf("unsupported input format for idempotency key")
	}

	err := d.request.ParseForm()
	if err != nil {
		return nil, err
	}

	formData := d.request.PostForm

	return d.extractFromFormValue(formData, parts)
}

// calculateChecksum generates a checksum using SHA256
func calculateChecksum(s string) string {
	// Create a new SHA256 hash object
	sha256Hash := sha256.New()

	// Convert the string to bytes and calculate the hash
	sha256Hash.Write([]byte(s))
	hashBytes := sha256Hash.Sum(nil)

	// Convert the hash bytes to a hexadecimal string
	hashString := hex.EncodeToString(hashBytes)

	return hashString
}
