package main

import ("fmt"; "os"; "strconv")

func main() {
 if len(os.Args) != 3 { os.Exit(1) }
 value, err := strconv.Atoi(os.Args[2]); if err != nil { os.Exit(1) }
 switch os.Args[1] {
 case "double", "halve": fmt.Println(value)
 case "square": fmt.Println(value * value)
 default: os.Exit(1)
 }
}
