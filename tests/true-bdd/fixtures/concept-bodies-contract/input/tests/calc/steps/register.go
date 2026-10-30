package steps

func Register(suite Registrar) {
	suite.Step("^the Owner halves (-?\\d+)$", halving)
	suite.Step("^the result is (-?\\d+)$", result)
	suite.Step("^the calculator runs on the \"([^\"]+)\" toolchain$", concept47f741283eda70d7a0d1d2aaa90ad59f9b70d6cc753b4652aced0fd106d24dd9)
}
