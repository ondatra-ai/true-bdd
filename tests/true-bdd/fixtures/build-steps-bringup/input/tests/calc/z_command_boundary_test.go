//go:build bdd

package calc_test

import (
 "os"
 "testing"
)

func TestDeclaredCommandWasRun(t *testing.T) {
 calls, err := os.ReadFile("../../tmp/calc-calls.log")
 if err != nil { t.Fatal(err) }
 if string(calls) != "double 3\n" { t.Fatalf("declared command calls: %q", calls) }
}
