module github.com/ondatra-ai/true-bdd

go 1.25.0

require (
	github.com/spf13/cobra v1.9.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
)

require github.com/ondatra-ai/true-bdd/pkg v0.0.0

replace github.com/ondatra-ai/true-bdd/pkg => ./pkg
