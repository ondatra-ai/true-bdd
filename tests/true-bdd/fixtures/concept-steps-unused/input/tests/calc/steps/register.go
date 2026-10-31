package steps

func Register(suite Registrar) {
 suite.Step(`^the result is (-?\d+)$`, result)
}
