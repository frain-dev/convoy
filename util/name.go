package util

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"math/rand"
	"strings"
	"time"
)

var firstNames = []string{"John", "Jane", "Alice", "Bob", "Charlie", "Mary", "Michael", "Emma", "Liam", "Olivia"}
var lastNames = []string{"Doe", "Smith", "Johnson", "Brown", "Williams", "Jones", "Garcia", "Martinez", "Davis", "Miller"}

// toSentenceCase converts a string to sentence case.
func toSentenceCase(name string) string {
	title := cases.Title(language.English)
	return title.String(strings.ToLower(name))
}

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// generateRandomName generates a random first and last name.
func generateRandomName() (string, string) {
	firstName := firstNames[rng.Intn(len(firstNames))]
	lastName := lastNames[rng.Intn(len(lastNames))]
	return firstName, lastName
}

// ExtractOrGenerateNamesFromEmail extracts first and last names from an email address or generates random names if necessary
func ExtractOrGenerateNamesFromEmail(email string) (string, string) {
	firstName, lastName := generateRandomName()
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return firstName, lastName
	}

	username := parts[0]
	nameParts := strings.FieldsFunc(username, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' '
	})

	if len(nameParts) > 0 {
		firstName = toSentenceCase(nameParts[0])
	}
	if len(nameParts) > 1 {
		lastName = toSentenceCase(strings.Join(nameParts[1:], " "))
	}

	return firstName, lastName
}
