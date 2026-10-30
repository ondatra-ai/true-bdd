package steps

type State struct {
	// CalcBinary is the absolute path of the calculator program the Given
	// built under the host's tmp directory.
	CalcBinary string
	// HostRoot is the absolute host root the calculator program is run from.
	HostRoot string
}
