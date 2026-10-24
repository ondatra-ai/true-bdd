package suite_test
import("testing";"fixture/buildcodefix/services/calc")
func TestA(t *testing.T){if calc.Mode()==1 {t.Skip("no execution result")};if calc.Mode()!=2 {t.Fatal("A needs mode two")}}
func TestB(t *testing.T){if calc.Mode()!=2 {t.Fatal("B needs mode two")}}
