package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	args := os.Args[1:]
	if len(args) < 2 {
		os.Exit(1)
	}

	operation, exact, rest := args[0], false, args[1:]
	if operation == "halve" && rest[0] == "--exact" {
		exact, rest = true, rest[1:]
	}

	if len(rest) != 1 {
		os.Exit(1)
	}

	value, err := strconv.Atoi(rest[0])
	if err != nil {
		os.Exit(1)
	}

	switch {
	case operation == "halve" && exact && value%2 != 0:
		fmt.Printf("%.1f\n", float64(value)/2)
	case operation == "halve":
		fmt.Println(value / 2)
	case operation == "double":
		fmt.Println(value * 2)
	default:
		os.Exit(1)
	}
}
