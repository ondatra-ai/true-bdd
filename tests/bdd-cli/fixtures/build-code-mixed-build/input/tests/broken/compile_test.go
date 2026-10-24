package broken_test
import("testing";"fixture/buildcodefix/services/calc")
func TestBuild(t *testing.T){if calc.B()!=2 {t.Fatal("B needs two")}}
