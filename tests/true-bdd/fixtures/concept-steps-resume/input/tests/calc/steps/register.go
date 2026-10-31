package steps

func Register(suite Registrar) {
	suite.Step(`^the result is (-?\d+)$`, result)

	suite.Step("^the Owner halves (-?\\d+)$", concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87)
}
