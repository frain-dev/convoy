package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ghodss/yaml"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
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
	const fname = "docs/swagger.json"
	f, err := os.Open(fname)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open "+fname)
	}
	defer f.Close()

	doc2 := &openapi2.T{}
	err = json.NewDecoder(f).Decode(doc2)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode "+fname)
	}

	return doc2, nil
}

func convertToV3(doc2 *openapi2.T) (*openapi3.T, error) {
	spec3, err := openapi2conv.ToV3(doc2)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate spec3")
	}

	return spec3, nil
}

const (
	spec3FnameJSON = "docs/v3/openapi3.json"
	spec3FnameYAML = "docs/v3/openapi3.yaml"
)

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

	err = os.WriteFile(spec3FnameJSON, indentBuf.Bytes(), 0644)
	if err != nil {
		return errors.Wrap(err, "failed to write "+spec3FnameJSON)
	}

	fmt.Println("created", spec3FnameJSON)

	yamlBuf, err := yaml.JSONToYAML(buf)
	if err != nil {
		return errors.Wrap(err, "failed to convert json docV3 to yaml")
	}

	err = os.WriteFile(spec3FnameYAML, yamlBuf, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to write "+spec3FnameYAML)
	}

	fmt.Println("created", spec3FnameYAML)

	return nil

}
