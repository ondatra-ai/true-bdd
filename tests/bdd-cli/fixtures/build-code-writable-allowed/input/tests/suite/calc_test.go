package suite_test

import (
 "testing"
 "fixture/writable/services/calc"
 "gopkg.in/yaml.v3"
)

func TestAdd(t *testing.T) {
 var numbers []int
 if err := yaml.Unmarshal([]byte("[2, 3]"), &numbers); err != nil { t.Fatal(err) }
 if got := calc.Add(numbers[0], numbers[1]); got != 5 { t.Fatalf("got %d, want 5", got) }
}
