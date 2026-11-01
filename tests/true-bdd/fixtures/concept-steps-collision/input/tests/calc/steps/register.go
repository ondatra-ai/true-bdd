package steps

func Register(suite Registrar) {
 suite.Step(`^the Owner halves (\d+)$`, halveNumber)
 suite.Step(`^the result is (-?\d+)$`, result)
}
