# Tiger Style Linting Rules

This document explains how the `.golangci.yml` configuration enforces the Tiger Style principles from [tigerstyle.dev](https://tigerstyle.dev/).

## Core Principles

### 1. Safety

#### Control and Limits

-   **Function Length**: Limited to 70 lines (`funlen: lines: 70`)
-   **Cyclomatic Complexity**: Max 10 per function (`cyclop: max-complexity: 10`)
-   **Package Functions**: Max 15 functions per package (`cyclop: max-functions: 15`)

#### Memory and Types

-   **Explicit Types**: Enforced by `staticcheck` and `gosimple`
-   **Variable Scope**: Minimized through `varcheck` and `unused`
-   **Memory Safety**: Checked by `gosec` and `govet`

#### Error Handling

-   **All Errors Must Be Handled**: Enforced by `errcheck`
-   **No Unused Variables**: Enforced by `unused` and `varcheck`
-   **Proper Error Wrapping**: Enforced by `wrapcheck`

### 2. Performance

#### Design for Performance

-   **Resource Optimization**: Enforced by `prealloc`, `unparam`, `wastedassign`
-   **Predictable Code**: Enforced by `staticcheck` and `gosimple`
-   **Efficient Patterns**: Enforced by `gocritic`

#### Efficient Resource Use

-   **Memory Allocation**: Checked by `prealloc`
-   **Unused Parameters**: Detected by `unparam`
-   **Wasted Assignments**: Caught by `wastedassign`

### 3. Developer Experience

#### Name Things

-   **Clear Naming**: Enforced by `revive` with custom rules
-   **Consistent Style**: Enforced by `gofmt` and `goimports`
-   **No Abbreviations**: Enforced by `revive` naming rules
-   **Units in Names**: Encouraged through documentation

#### Organize Things

-   **Logical Structure**: Enforced by `revive` and `staticcheck`
-   **Simple Interfaces**: Enforced by `unparam` and `gocritic`
-   **Consistent Formatting**: Enforced by `gofmt`, `goimports`, `whitespace`

#### Ensure Consistency

-   **No Duplicates**: Enforced by `dupl`
-   **Consistent Indentation**: Enforced by `gofmt` and `whitespace`
-   **Line Length**: Limited to 100 characters (`lll: line-length: 100`)
-   **Standardized Tooling**: Enforced through consistent linting rules

## Specific Linter Mappings

### Safety Linters

-   `cyclop`: Control flow complexity
-   `funlen`: Function length limits
-   `errcheck`: Error handling
-   `govet`: Go vet checks
-   `gosimple`: Code simplification
-   `staticcheck`: Static analysis
-   `gocritic`: Code quality checks
-   `gosec`: Security checks

### Performance Linters

-   `prealloc`: Pre-allocate slices
-   `unparam`: Unused parameters
-   `wastedassign`: Wasted assignments
-   `unconvert`: Unnecessary conversions

### Developer Experience Linters

-   `revive`: Code style and naming
-   `gofmt`: Code formatting
-   `goimports`: Import organization
-   `lll`: Line length limits
-   `misspell`: Spelling checks
-   `whitespace`: Whitespace consistency
-   `dupl`: Duplicate code detection

## Exclusions and Exceptions

### Test Files

-   Function length limits relaxed for test files
-   Complexity limits relaxed for test files
-   All linters disabled for test data directories

### Generated Code

-   All linters disabled for generated code
-   Mock files excluded from most checks

### Command Files

-   Function length limits relaxed for main command files
-   Migration files have relaxed constraints

## Usage

### Local Development

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linting
golangci-lint run

# Run with specific linters
golangci-lint run --enable=revive,staticcheck

# Fix auto-fixable issues
golangci-lint run --fix
```

### CI/CD Integration

The configuration is already integrated into the GitHub Actions workflow at `.github/workflows/linter.yml`.

### IDE Integration

Most Go IDEs support golangci-lint integration. Configure your IDE to use the `.golangci.yml` file for consistent linting across all environments.

## Custom Rules

### Tiger Style Specific

1. **Function Length**: Strict 70-line limit for production code
2. **Complexity**: Maximum cyclomatic complexity of 10
3. **Error Handling**: All errors must be explicitly handled
4. **Naming**: Descriptive names with units/qualifiers when appropriate
5. **Consistency**: Uniform formatting and style across the codebase

### Performance Guidelines

1. **Resource Optimization**: Pre-allocate slices, avoid wasted assignments
2. **Predictable Code**: Use explicit types, avoid magic numbers
3. **Efficient Patterns**: Follow Go best practices for performance

### Developer Experience

1. **Readability**: Clear, consistent code formatting
2. **Maintainability**: Logical organization and structure
3. **Collaboration**: Standardized tooling and practices

## Compliance

This configuration enforces Tiger Style principles by:

-   Treating all warnings as errors
-   Setting strict limits on function complexity and length
-   Enforcing comprehensive error handling
-   Maintaining a consistent code style and formatting
-   Promoting performance-conscious coding practices

The goal is to create code that is safe, performant, and enjoyable to work with, following the core principles outlined in the Tiger Style guide.
