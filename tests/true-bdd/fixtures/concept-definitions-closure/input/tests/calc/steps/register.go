package steps

func Register(suite Registrar) {
	suite.Step(`^the Owner halves (-?\d+)$`, halving)
	suite.Step(`^the Owner doubles (\d+)$`, doubling)
	suite.Step(`^the Owner doubles 6$`, doublingSix)
	suite.Step(`^the result is (-?\d+)$`, result)
	suite.Step(`^the result is even$`, even)
}
