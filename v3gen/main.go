package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/frain-dev/convoy/pkg/log"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

const (
	v2Fame      = "docs/swagger.json"
	v3FnameJSON = "docs/v3/openapi3.json"
	v3FnameYAML = "docs/v3/openapi3.yaml"
)

func main() {
	docV2, err := loadV2()
	if err != nil {
		log.WithError(err).Fatal("loadV2 failed")
	}

	docV3, err := convertToV3(docV2)
	if err != nil {
		log.WithError(err).Fatal("convertToV3 failed")
	}

	err = writeOutDocV3(docV3)
	if err != nil {
		log.WithError(err).Fatal("writeOutDocV3 failed")
	}
}

func loadV2() (*openapi2.T, error) {
	f, err := os.Open(v2Fame)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open "+v2Fame)
	}
	defer f.Close()

	docV2 := &openapi2.T{}
	err = json.NewDecoder(f).Decode(docV2)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode "+v2Fame)
	}

	return docV2, nil
}

func convertToV3(docV2 *openapi2.T) (*openapi3.T, error) {
	docV3, err := openapi2conv.ToV3(docV2)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate docV3")
	}

	return docV3, nil
}

func writeOutDocV3(docV3 *openapi3.T) error {
	buf, err := docV3.MarshalJSON()
	if err != nil {
		return errors.Wrap(err, "failed to marshal spec3")
	}

	indentBuf := &bytes.Buffer{}
	err = json.Indent(indentBuf, buf, "", "	")
	if err != nil {
		return errors.Wrap(err, "failed to indent docV3 json")
	}

	err = os.WriteFile(v3FnameJSON, indentBuf.Bytes(), 0644)
	if err != nil {
		return errors.Wrap(err, "failed to write "+v3FnameJSON)
	}

	fmt.Println("created", v3FnameJSON)

	yamlBuf, err := yaml.JSONToYAML(buf)
	if err != nil {
		return errors.Wrap(err, "failed to convert json docV3 to yaml")
	}

	err = os.WriteFile(v3FnameYAML, yamlBuf, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to write "+v3FnameYAML)
	}

	fmt.Println("created", v3FnameYAML)

	return nil

}
