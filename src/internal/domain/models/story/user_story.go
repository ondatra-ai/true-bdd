package story

type UserStory struct {
	AsA    string `json:"as_a"    yaml:"as_a"`
	IWant  string `json:"i_want"  yaml:"i_want"`
	SoThat string `json:"so_that" yaml:"so_that"`
}
