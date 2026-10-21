// Package greeting holds one function the sample half runs and one it never
// calls.
package greeting

// Greeter carries the methods, so a report names a receiver.
type Greeter struct{ name string }

// Greet greets name, or nobody when name is empty.
func Greet(name string) string {
	if name == "" {
		name = "nobody"
	}

	return Greeter{name: name}.Hello()
}

// Hello is the greeting itself.
func (g Greeter) Hello() string {
	return "hello " + g.name
}

// Farewell is what main says when given nothing to greet.
func (g *Greeter) Farewell() string {
	if g.name == "" {
		return "bye"
	}

	return "bye " + g.name
}
