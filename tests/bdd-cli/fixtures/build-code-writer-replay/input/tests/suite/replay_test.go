package suite_test

import (
 "flag"
 "os"
 "testing"
 "time"

 "fixture/buildcodefix/services/calc"
)

func TestSlowReplay(t *testing.T) {
 if calc.Add(2, 3) != 5 { t.Fatal("Add must sum its arguments") }
 if flag.Lookup("test.run").Value.String() != "^TestSlowReplay$" { return }
 const marker = "../../services/calc/replayed.txt"
 if _, err := os.Stat(marker); err == nil { return }
 time.Sleep(65 * time.Second)
 if err := os.WriteFile(marker, []byte("SELECTION COMPLETED\n"), 0o644); err != nil { t.Fatal(err) }
 t.Log("SELECTION COMPLETED")
}
