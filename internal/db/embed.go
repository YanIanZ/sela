package db

import "embed"

//go:embed migrations/*.sql
var embedded embed.FS

// Embedded parses the migrations compiled into this binary.
func Embedded() ([]Migration, error) { return Parse(embedded, "migrations") }

// MustEmbedded is Embedded for program start and tests; the embedded set
// is validated by tests, so a failure here is a build defect.
func MustEmbedded() []Migration {
	m, err := Embedded()
	if err != nil {
		panic(err)
	}
	return m
}
