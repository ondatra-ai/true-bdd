package suite_test
import("testing";"fixture/buildcodefix/services/calc")
func TestA(t *testing.T){if calc.A()!=1 {t.Fatal("SHARED-FAILURE-EVIDENCE")}}
func TestB(t *testing.T){if calc.B()!=2 {t.Fatal("SHARED-FAILURE-EVIDENCE")}}
