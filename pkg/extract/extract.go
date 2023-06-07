package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
)

type person struct {
	Age int `json:"age"`
}

func ExtractData(request *http.Request, input []string) (interface{}, error) {
	var data []interface{}

	for _, s := range input {
		parts := strings.Split(s, ".")

		switch parts[0] {
		case "request":
			d, err := extractFromRequest(request, parts[1:])
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

func extractFromRequest(request *http.Request, parts []string) (interface{}, error) {
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid input format")
	}

	switch parts[0] {
	case "Header":
		return extractFromHeader(request.Header, parts[1:])
	case "Body":
		contentType := request.Header.Get("Content-Type")
		switch {
		case strings.HasPrefix(contentType, "application/json"):
			return extractFromBodyJSON(request, parts[1:])
		case strings.HasPrefix(contentType, "multipart/form-data"):
			return extractFromBodyFormData(request, parts[1:])
		case strings.HasPrefix(contentType, "application/x-www-form-urlencoded"):
			return extractFromBodyURLEncoded(request, parts[1:])
		default:
			return nil, fmt.Errorf("unsupported request body format: %s", contentType)
		}
	case "QueryParam":
		return extractFromQuery(request.URL.Query(), parts[1:])
	default:
		return nil, fmt.Errorf("unsupported input format")
	}
}

func extractFromHeader(header http.Header, parts []string) (interface{}, error) {
	if len(parts) != 1 {
		return nil, fmt.Errorf("invalid input format")
	}

	return header.Get(parts[0]), nil
}

func extractFromBodyJSON(request *http.Request, parts []string) (interface{}, error) {
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

	return extractFromJSON(jsonData, parts)
}

func extractFromJSON(jsonData map[string]interface{}, parts []string) (interface{}, error) {
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

	return extractFromJSON(subJSON, parts[1:])
}

func extractFromQuery(queryParams url.Values, parts []string) (interface{}, error) {
	if len(parts) != 1 {
		return nil, fmt.Errorf("invalid input format")
	}

	values, ok := queryParams[parts[0]]
	if !ok {
		return nil, fmt.Errorf("query parameter not found")
	}

	if len(values) > 0 {
		return values[0], nil
	}

	return nil, fmt.Errorf("query parameter is empty")
}

func extractFromBodyFormData(request *http.Request, parts []string) (interface{}, error) {
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid input format")
	}

	err := request.ParseMultipartForm(32 << 20) // 32 MB
	if err != nil {
		return nil, err
	}

	formData := request.MultipartForm.Value

	return extractFromFormValue(formData, parts)
}

func extractFromFormValue(form url.Values, parts []string) (interface{}, error) {
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid input format")
	}

	// Perform a depth-first search to find the nested field
	nestedValue, found := performDFS(form, parts)
	if !found {
		return nil, fmt.Errorf("nested field not found")
	}

	return nestedValue, nil
}

func performDFS(form url.Values, parts []string) (interface{}, bool) {
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
	part, index := parsePartIndex(currentPart)
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
	return performDFS(nestedForm, parts[1:])
}

func parsePartIndex(part string) (string, int) {
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

func extractFromBodyURLEncoded(request *http.Request, parts []string) (interface{}, error) {
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid input format")
	}

	err := request.ParseForm()
	if err != nil {
		return nil, err
	}

	formData := request.PostForm

	return extractFromFormValue(formData, parts)
}

func QueryParam() {
	////////////////////////////////////////////////////////////////////////////
	//
	// QueryParam
	//
	////////////////////////////////////////////////////////////////////////////

	request, err := http.NewRequest("GET", "https://example.com/?name=John&age=25", nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	result, err := ExtractData(request, []string{"request.QueryParam.name"})
	if err != nil {
		fmt.Println("Error extracting data:", err)
		return
	}

	fmt.Println(result) // Output: John
}

func Header() {
	////////////////////////////////////////////////////////////////////////////
	//
	// Header
	//
	////////////////////////////////////////////////////////////////////////////

	// Example for extracting data from the request header
	requestHeader := http.Header{}
	requestHeader.Add("Authorization", "Bearer myToken123")
	request, err := http.NewRequest("GET", "https://example.com", nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	request.Header = requestHeader

	result, err := ExtractData(request, []string{"request.Header.Authorization"})
	if err != nil {
		fmt.Println("Error extracting data:", err)
		return
	}

	fmt.Println(result) // Output: Bearer myToken123
}

func RequestBody() {
	////////////////////////////////////////////////////////////////////////////
	//
	// Body (JSON)
	//
	////////////////////////////////////////////////////////////////////////////

	// Example for extracting data from the request body
	person := person{
		Age: 25,
	}
	body, err := json.Marshal(person)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}
	request, err := http.NewRequest("POST", "https://example.com", strings.NewReader(string(body)))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	request.Header.Set("Content-Type", "application/json")

	result, err := ExtractData(request, []string{"request.Body.age"})
	if err != nil {
		fmt.Println("Error extracting data:", err)
		return
	}

	fmt.Println(result) // Output: 25
}

func RequestBodyFormData() {
	////////////////////////////////////////////////////////////////////////////
	//
	// Body (Form Data)
	//
	////////////////////////////////////////////////////////////////////////////

	// Example for extracting data from a form-data request body
	b := &bytes.Buffer{}
	writer := multipart.NewWriter(b)
	err := writer.WriteField("name", "John")
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	err = writer.Close()
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	request, err := http.NewRequest("POST", "https://example.com", b)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	result, err := ExtractData(request, []string{"request.Body.name"})
	if err != nil {
		fmt.Println("Error extracting data:", err)
		return
	}

	fmt.Println(result) // Output: John
}

func RequestBodyUrlEncoded() {
	////////////////////////////////////////////////////////////////////////////
	//
	// Body (Url Encoded)
	//
	////////////////////////////////////////////////////////////////////////////

	// Example for extracting data from an x-www-form-urlencoded request body
	request, err := http.NewRequest("POST", "https://example.com", strings.NewReader("age=25"))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	result, err := ExtractData(request, []string{"request.Body.age"})
	if err != nil {
		fmt.Println("Error extracting data:", err)
		return
	}

	fmt.Println(result) // Output: 25
}

func RequestBodyFormDataNested() {
	type Person struct {
		Name    string
		Age     int
		Address struct {
			Street string
			City   string
			Zip    string
		}
	}

	// Create a new person with nested data
	person := Person{
		Name: "John",
		Age:  25,
		Address: struct {
			Street string
			City   string
			Zip    string
		}{
			Street: "123 Main St",
			City:   "New York",
			Zip:    "10001",
		},
	}

	// Create a buffer to store the form data
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// Set the Content-Type header
	contentType := writer.FormDataContentType()

	// Write the person's name
	fieldName := "name"
	fieldValue := person.Name
	writer.WriteField(fieldName, fieldValue)

	// Write the person's age
	fieldName = "age"
	fieldValue = fmt.Sprintf("%d", person.Age)
	writer.WriteField(fieldName, fieldValue)

	// Write the address fields
	addressPrefix := "address"
	addressFields := []struct {
		Name  string
		Value string
	}{
		{Name: "street", Value: person.Address.Street},
		{Name: "city", Value: person.Address.City},
		{Name: "zip", Value: person.Address.Zip},
	}

	for _, field := range addressFields {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s[%s]"`, addressPrefix, field.Name))
		part, _ := writer.CreatePart(h)
		part.Write([]byte(field.Value))
	}

	// Close the multipart writer
	writer.Close()

	// Create a new HTTP request with the form data
	request, err := http.NewRequest("POST", "https://example.com", body)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	request.Header.Set("Content-Type", contentType)

	// Extract data from the request
	result, err := ExtractData(request, []string{"request.Body.address[zip]", "request.Body.address[city]"})
	if err != nil {
		fmt.Println("Error extracting data:", err)
		return
	}

	fmt.Println(result) // Output: 10001
}
