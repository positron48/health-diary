package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "status") {
		fmt.Fprintln(os.Stderr, "usage: health-diary-migrate <up|status>")
		os.Exit(2)
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}
	m, err := migrate.New("file:///app/migrations", databaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open migrations:", err)
		os.Exit(1)
	}
	defer m.Close()
	if os.Args[1] == "status" {
		version, dirty, err := m.Version()
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Println("version: none")
			return
		}
		if err != nil {
			panic(err)
		}
		fmt.Printf("version: %d dirty: %t\n", version, dirty)
		return
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintln(os.Stderr, "apply migrations:", err)
		os.Exit(1)
	}
	fmt.Println("migrations applied")
}
