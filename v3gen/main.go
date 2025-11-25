package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"

	"github.com/frain-dev/convoy/pkg/log"
)

const (
	v2Fame      = "docs/swagger.json"
	v3FnameJSON = "docs/v3/openapi3.json"
	v3FnameYAML = "docs/v3/openapi3.yaml"
)

func main() {
	var l = log.NewLogger(os.Stdout)
	docV2, err := loadV2()
	if err != nil {
		l.Fatal("loadV2 failed: " + err.Error())
	}

	docV3, err := convertToV3(docV2)
	if err != nil {
		l.Fatal("convertToV3 failed: " + err.Error())
	}

	err = writeOutDocV3(docV3)
	if err != nil {
		l.Fatal("writeOutDocV3 failed: " + err.Error())
	}
}

func loadV2() (*openapi2.T, error) {
	f, err := os.Open(v2Fame)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open "+v2Fame)
	}
	defer func() {
		_ = f.Close()
	}()

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

	jsonFilePath := filepath.Clean(filepath.Join(findProjectRoot(), v3FnameJSON))

	fJson, err := os.OpenFile(jsonFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errors.Wrap(err, "failed to open "+v3FnameJSON)
	}

	_, err = fJson.Write(indentBuf.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to write "+v3FnameJSON)
	}

	yamlBuf, err := yaml.JSONToYAML(buf)
	if err != nil {
		return errors.Wrap(err, "failed to convert json docV3 to yaml")
	}

	yamlFilePath := filepath.Clean(filepath.Join(findProjectRoot(), v3FnameYAML))
	fYaml, err := os.OpenFile(yamlFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errors.Wrap(err, "failed to open "+v3FnameJSON)
	}

	_, err = fYaml.Write(yamlBuf)
	if err != nil {
		return errors.Wrap(err, "failed to write "+v3FnameYAML)
	}

	fmt.Println("created", v3FnameYAML)

	return nil
}

func findProjectRoot() (roots string) {
	cwd, _ := os.Getwd()
	dir := filepath.Clean(cwd)

	// Look for enclosing go.mod.
	for {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
		d := filepath.Dir(dir)
		if d == dir {
			break
		}
		dir = d
	}
	return ""
}
