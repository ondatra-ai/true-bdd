package suite_test
import("testing";"fixture/buildcodefix/services/calc")
func TestA(t *testing.T){if calc.A()!=1 {t.Fatal("A needs one")}}
