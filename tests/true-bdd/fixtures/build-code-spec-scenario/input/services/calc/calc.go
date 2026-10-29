package main

import ("fmt"; "os"; "strconv")

func main() {
 if len(os.Args) != 3 || os.Args[1] != "double" { os.Exit(1) }
 value, err := strconv.Atoi(os.Args[2]); if err != nil { os.Exit(1) }
 fmt.Println(value)
}
