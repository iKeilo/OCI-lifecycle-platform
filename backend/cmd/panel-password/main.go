package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"a-series-oracle/backend/internal/auth"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "hash" {
		fmt.Fprintln(os.Stderr, "usage: panel-password hash < password.txt")
		os.Exit(2)
	}
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 4096))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	password := strings.TrimSpace(string(raw))
	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(hash)
}
