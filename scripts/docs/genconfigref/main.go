// Command genconfigref walks config.Configuration (struct tags + config.DefaultConfiguration)
// and writes a deterministic JSON reference of JSON key paths, env vars, and defaults.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/frain-dev/convoy/config"
)

type entry struct {
	JSONPath string `json:"json_path"`
	EnvVar   string `json:"env_var"`
	GoType   string `json:"go_type"`
	Default  string `json:"default"`
}

type outputDoc struct {
	Source  string  `json:"source"`
	Entries []entry `json:"entries"`
}

func main() {
	outPath := flag.String("output", "", "write JSON to this path (default: stdout)")
	flag.Parse()

	var entries []entry
	walkStruct(reflect.TypeOf(config.Configuration{}), reflect.ValueOf(config.DefaultConfiguration), "", &entries)

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].EnvVar != entries[j].EnvVar {
			return entries[i].EnvVar < entries[j].EnvVar
		}
		return entries[i].JSONPath < entries[j].JSONPath
	})

	doc := outputDoc{
		Source:  "github.com/frain-dev/convoy/config.DefaultConfiguration",
		Entries: entries,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}

	data := bytes.TrimSpace(buf.Bytes())
	if *outPath != "" {
		if err := os.WriteFile(*outPath, append(data, '\n'), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", *outPath, err)
			os.Exit(1)
		}
		return
	}
	os.Stdout.Write(append(data, '\n'))
}

func walkStruct(typ reflect.Type, val reflect.Value, pathPrefix string, out *[]entry) {
	typ, val = deref(typ, val)
	if typ.Kind() != reflect.Struct {
		return
	}
	if typ == reflect.TypeOf(time.Time{}) {
		return
	}

	for i := 0; i < typ.NumField(); i++ {
		sf := typ.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		seg := jsonPathSegment(typ, sf)
		if seg == "" {
			continue
		}
		fieldPath := seg
		if pathPrefix != "" {
			fieldPath = pathPrefix + "." + seg
		}

		fv := val.Field(i)
		ft := sf.Type
		env := strings.TrimSpace(sf.Tag.Get("envconfig"))
		if env == "-" {
			env = ""
		}

		ftd, fvd := deref(ft, fv)

		if env != "" {
			*out = append(*out, entry{
				JSONPath: fieldPath,
				EnvVar:   env,
				GoType:   ft.String(),
				Default:  formatDefault(fvd, ftd),
			})
		}

		switch ftd.Kind() {
		case reflect.Struct:
			if ftd != reflect.TypeOf(time.Time{}) {
				walkStruct(ftd, fvd, fieldPath, out)
			}
		case reflect.Slice, reflect.Array:
			// env-backed slices are documented as a single row; do not descend
		default:
		}
	}
}

func deref(typ reflect.Type, val reflect.Value) (reflect.Type, reflect.Value) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if val.Kind() == reflect.Pointer {
			if val.IsNil() {
				val = reflect.Zero(typ)
			} else {
				val = val.Elem()
			}
		}
	}
	return typ, val
}

func jsonPathSegment(structType reflect.Type, sf reflect.StructField) string {
	j := strings.Split(sf.Tag.Get("json"), ",")[0]
	switch j {
	case "-":
		if s := siblingStrJSONSegment(structType, sf); s != "" {
			return s
		}
		return camelToSnakePath(sf.Name)
	case "":
		return camelToSnakePath(sf.Name)
	default:
		return j
	}
}

func siblingStrJSONSegment(structType reflect.Type, sf reflect.StructField) string {
	strName := sf.Name + "Str"
	f, ok := structType.FieldByName(strName)
	if !ok {
		return ""
	}
	jj := strings.Split(f.Tag.Get("json"), ",")[0]
	if jj == "" || jj == "-" {
		return ""
	}
	return jj
}

func camelToSnakePath(name string) string {
	var b strings.Builder
	for i, r := range name {
		if i > 0 && unicode.IsUpper(r) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func formatDefault(v reflect.Value, t reflect.Type) string {
	if !v.IsValid() {
		return ""
	}
	switch t.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		if v.Bool() {
			return "true"
		}
		return "false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if t.PkgPath() == "time" && t.Name() == "Duration" {
			return time.Duration(v.Int()).String()
		}
		return fmt.Sprintf("%d", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return fmt.Sprintf("%d", v.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", v.Float())
	default:
		b, err := json.Marshal(v.Interface())
		if err != nil {
			return fmt.Sprintf("%v", v.Interface())
		}
		return string(b)
	}
}
