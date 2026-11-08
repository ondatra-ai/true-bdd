package main

import ("fmt"; "os"; "strconv")

func main() {
 if len(os.Args) != 3 || os.Args[1] != "double" { os.Exit(1) }
 value, err := strconv.Atoi(os.Args[2]); if err != nil { os.Exit(1) }
 if err := os.MkdirAll("tmp", 0755); err != nil { os.Exit(1) }
 calls, err := os.OpenFile("tmp/calc-calls.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
 if err != nil { os.Exit(1) }
 if _, err := fmt.Fprintf(calls, "double %d\n", value); err != nil { os.Exit(1) }
 if err := calls.Close(); err != nil { os.Exit(1) }
 fmt.Println(value * 2)
}
