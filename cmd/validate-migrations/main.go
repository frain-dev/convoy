package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MigrationBlock represents a single -- +migrate Up/Down block
type MigrationBlock struct {
	FilePath         string
	BlockNum         int
	StartLine        int
	Direction        string
	HasNotransaction bool
	IndexOperations  []Operation
	OtherDDL         []Operation
}

// Operation represents a SQL operation with its line number
type Operation struct {
	LineNum   int
	Statement string
}

// IsMixed checks if block has both index operations and other DDL
func (b *MigrationBlock) IsMixed() bool {
	return len(b.IndexOperations) > 0 && len(b.OtherDDL) > 0
}

var (
	// Regex patterns
	migrateDirectiveRegex = regexp.MustCompile(`(?i)--\s*\+migrate\s+(Up|Down)(\s+notransaction)?`)
	indexOperationRegex   = regexp.MustCompile(`(?i)\b(CREATE|DROP)\s+(UNIQUE\s+)?INDEX\b`)
	reindexRegex          = regexp.MustCompile(`(?i)\bREINDEX\s+(INDEX|TABLE)\b`)
	setStatementRegex     = regexp.MustCompile(`(?i)^SET\s+`)

	// DDL patterns to check
	ddlPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bALTER\s+TABLE\b`),
		regexp.MustCompile(`(?i)\bCREATE\s+TABLE\b`),
		regexp.MustCompile(`(?i)\bDROP\s+TABLE\b`),
		regexp.MustCompile(`(?i)\bTRUNCATE\b`),
		regexp.MustCompile(`(?i)\bALTER\s+COLUMN\b`),
		regexp.MustCompile(`(?i)\bADD\s+COLUMN\b`),
		regexp.MustCompile(`(?i)\bDROP\s+COLUMN\b`),
		regexp.MustCompile(`(?i)\bADD\s+CONSTRAINT\b`),
		regexp.MustCompile(`(?i)\bDROP\s+CONSTRAINT\b`),
		regexp.MustCompile(`(?i)\bCREATE\s+TYPE\b`),
		regexp.MustCompile(`(?i)\bCREATE\s+FUNCTION\b`),
		regexp.MustCompile(`(?i)\bCREATE\s+TRIGGER\b`),
		regexp.MustCompile(`(?i)\bVALIDATE\s+CONSTRAINT\b`),
	}
)

// parseMigrationFile parses a SQL migration file into blocks
func parseMigrationFile(filePath string) ([]MigrationBlock, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var blocks []MigrationBlock
	var currentBlock *MigrationBlock
	blockNum := 0

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		stripped := strings.TrimSpace(line)

		// Check for migration directive
		if matches := migrateDirectiveRegex.FindStringSubmatch(stripped); matches != nil {
			// Save previous block
			if currentBlock != nil {
				blocks = append(blocks, *currentBlock)
			}

			// Start new block
			blockNum++
			direction := matches[1]
			hasNotransaction := len(matches) > 2 && matches[2] != ""

			currentBlock = &MigrationBlock{
				FilePath:         filePath,
				BlockNum:         blockNum,
				StartLine:        lineNum,
				Direction:        direction,
				HasNotransaction: hasNotransaction,
				IndexOperations:  []Operation{},
				OtherDDL:         []Operation{},
			}
			continue
		}

		// Skip if not in a block or line is empty/comment
		if currentBlock == nil || stripped == "" || strings.HasPrefix(stripped, "--") {
			continue
		}

		// Check for index operations (CREATE/DROP INDEX, REINDEX)
		if indexOperationRegex.MatchString(stripped) || reindexRegex.MatchString(stripped) {
			currentBlock.IndexOperations = append(currentBlock.IndexOperations, Operation{
				LineNum:   lineNum,
				Statement: stripped,
			})
			continue
		}

		// Skip SET statements
		if setStatementRegex.MatchString(stripped) {
			continue
		}

		// Check for other DDL operations
		for _, pattern := range ddlPatterns {
			if pattern.MatchString(stripped) {
				currentBlock.OtherDDL = append(currentBlock.OtherDDL, Operation{
					LineNum:   lineNum,
					Statement: stripped,
				})
				break
			}
		}
	}

	// Save last block
	if currentBlock != nil {
		blocks = append(blocks, *currentBlock)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return blocks, nil
}

// validateMigrations validates all migration files in a directory
func validateMigrations(sqlDir string) ([]string, int, error) {
	var errors []string
	totalViolations := 0

	files, err := filepath.Glob(filepath.Join(sqlDir, "*.sql"))
	if err != nil {
		return nil, 0, err
	}

	for _, filePath := range files {
		blocks, err := parseMigrationFile(filePath)
		if err != nil {
			return nil, 0, fmt.Errorf("error parsing %s: %w", filePath, err)
		}

		for _, block := range blocks {
			if !block.IsMixed() {
				continue
			}

			totalViolations++
			fileName := filepath.Base(filePath)

			errorMsg := fmt.Sprintf("\n❌ %s:%d (Block %d, %s)",
				fileName, block.StartLine, block.BlockNum, block.Direction)
			errorMsg += "\n   Mixed index operations with other DDL (requires separate migration blocks)\n"

			errorMsg += "\n   Index operations:"
			for _, op := range block.IndexOperations {
				stmt := op.Statement
				if len(stmt) > 80 {
					stmt = stmt[:80] + "..."
				}
				errorMsg += fmt.Sprintf("\n     Line %d: %s", op.LineNum, stmt)
			}

			errorMsg += "\n\n   Other DDL operations:"
			for _, op := range block.OtherDDL {
				stmt := op.Statement
				if len(stmt) > 80 {
					stmt = stmt[:80] + "..."
				}
				errorMsg += fmt.Sprintf("\n     Line %d: %s", op.LineNum, stmt)
			}

			errorMsg += "\n\n   💡 Solution: Split into separate migration blocks:"
			errorMsg += "\n      - Keep index operations in a migration block with 'notransaction'"
			errorMsg += "\n      - Move other DDL to a separate migration block (without 'notransaction')\n"

			errors = append(errors, errorMsg)
		}
	}

	return errors, totalViolations, nil
}

func main() {
	// Get SQL directory path
	sqlDir := "sql"
	if len(os.Args) > 1 {
		sqlDir = os.Args[1]
	}

	// Check if directory exists
	if _, err := os.Stat(sqlDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: SQL directory not found at %s\n", sqlDir)
		os.Exit(1)
	}

	// Count files
	files, _ := filepath.Glob(filepath.Join(sqlDir, "*.sql"))
	fileCount := len(files)

	fmt.Println("🔍 Validating SQL migrations for mixed index/DDL operations...")
	fmt.Printf("   Checking %d migration files\n\n", fileCount)

	errors, totalViolations, err := validateMigrations(sqlDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(errors) > 0 {
		fmt.Printf("Found %d migration blocks with mixed operations:\n", totalViolations)
		for _, errorMsg := range errors {
			fmt.Println(errorMsg)
		}

		fmt.Println("\n" + strings.Repeat("=", 80))
		fmt.Printf("❌ Validation failed: %d violations found\n", totalViolations)
		fmt.Println(strings.Repeat("=", 80))
		os.Exit(1)
	}

	fmt.Println("✅ All migrations validated successfully!")
	fmt.Println("   No mixed index/DDL operations found")
	os.Exit(0)
}
