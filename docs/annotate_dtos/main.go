// tools/annotate_dtos/main.go
//
//nolint:all
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	dryRun := len(os.Args) > 1 && os.Args[1] == "--dry-run"

	// api/models: only *Response structs (request DTO nullability is owned by
	// validation). datastore: every struct, because responses embed datastore
	// types whose pointer/null fields serialize as JSON null; without
	// x-nullable the generated SDK parsers reject those nulls.
	dirs := []struct {
		path         string
		responseOnly bool
	}{
		{"./api/models", true},
		{"./datastore", false},
	}

	for _, dir := range dirs {
		fmt.Printf("Using AST to add nullable extensions in: %s\n", dir.path)

		err := filepath.Walk(dir.path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Walk reports lstat info, so symlinks are visible here. Skip them
			// so reads/writes cannot be redirected outside the repo tree.
			if info.Mode()&os.ModeSymlink != 0 {
				return nil
			}

			if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				processFileWithAST(path, dir.responseOnly, dryRun)
			}
			return nil
		})

		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}

func processFileWithAST(filePath string, responseOnly bool, dryRun bool) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", filePath, err)
		return
	}

	// Check if file contains Response structs
	if responseOnly && !strings.Contains(string(content), "Response") {
		return
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		fmt.Printf("Error parsing %s: %v\n", filePath, err)
		return
	}

	var fieldsToFix []fieldInfo

	ast.Inspect(node, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		// Only process Response structs when scoped to responses
		if responseOnly && !strings.Contains(typeSpec.Name.Name, "Response") {
			return true
		}

		for _, field := range structType.Fields.List {
			if field.Tag == nil || len(field.Names) == 0 {
				continue
			}

			tagValue := field.Tag.Value
			// Remove backticks
			tagValue = strings.Trim(tagValue, "`")

			// Skip if already has extensions:"x-nullable"
			if strings.Contains(tagValue, `extensions:"x-nullable"`) {
				continue
			}

			// Check if field type is nullable
			fieldType := getFieldTypeString(field.Type)
			if isNullableTypeAST(fieldType) {
				pos := fset.Position(field.Pos())
				fieldsToFix = append(fieldsToFix, fieldInfo{
					name:     field.Names[0].Name,
					typeStr:  fieldType,
					tag:      tagValue,
					line:     pos.Line,
					filename: filePath,
				})
			}
		}
		return true
	})

	if len(fieldsToFix) == 0 {
		return
	}

	if dryRun {
		fmt.Printf("[DRY] %s: %d fields need extensions:\"x-nullable\"\n",
			filepath.Base(filePath), len(fieldsToFix))
		for _, field := range fieldsToFix {
			fmt.Printf("  - %s (%s)\n", field.name, field.typeStr)
		}
		return
	}

	// Actually fix the file
	lines := strings.Split(string(content), "\n")
	modified := false

	for _, field := range fieldsToFix {
		lineIndex := field.line - 1
		if lineIndex >= len(lines) {
			continue
		}

		line := lines[lineIndex]

		// Find the tag and add extensions:"x-nullable"
		tagStart := strings.Index(line, "`")
		tagEnd := strings.LastIndex(line, "`")
		if tagStart == -1 || tagEnd == -1 {
			continue
		}

		tagContent := line[tagStart+1 : tagEnd]
		newTagContent := tagContent + ` extensions:"x-nullable"`
		newLine := line[:tagStart+1] + newTagContent + line[tagEnd:]

		lines[lineIndex] = newLine
		modified = true

		fmt.Printf("✓ %s: %s\n", filepath.Base(filePath), field.name)
	}

	if modified {
		output := strings.Join(lines, "\n")
		if err := ioutil.WriteFile(filePath, []byte(output), 0644); err != nil {
			fmt.Printf("Error writing %s: %v\n", filePath, err)
		}
	}
}

type fieldInfo struct {
	name     string
	typeStr  string
	tag      string
	line     int
	filename string
}

func getFieldTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		xIdent, ok := t.X.(*ast.Ident)
		if !ok {
			return ""
		}
		return fmt.Sprintf("%s.%s", xIdent.Name, t.Sel.Name)
	case *ast.StarExpr:
		return "*" + getFieldTypeString(t.X)
	case *ast.ArrayType:
		return "[]" + getFieldTypeString(t.Elt)
	default:
		return ""
	}
}

func isNullableTypeAST(typeStr string) bool {
	// Check for null package types
	if strings.HasPrefix(typeStr, "null.") {
		return true
	}

	// Check for sql.Null types
	if strings.HasPrefix(typeStr, "sql.Null") {
		return true
	}

	// Check for pointer types
	if strings.HasPrefix(typeStr, "*") {
		// Don't treat pointers to slices/maps as nullable for JSON
		if strings.HasPrefix(typeStr, "*[]") || strings.HasPrefix(typeStr, "*map[") {
			return false
		}
		return true
	}

	return false
}
