// Command sample stands in for the engine: run with an argument it greets,
// so main's last statement and Farewell never run.
package main

import (
	"os"

	"example.com/sample/greeting"
)

func main() {
	if len(os.Args) > 1 {
		greeting.Greet(os.Args[1])

		return
	}

	new(greeting.Greeter).Farewell()
}
