package promptmodel

import "sync"

// Validator lazily compiles one host scheme and shares it across concurrent Cells.
type Validator struct {
	once   sync.Once
	scheme *Scheme
	err    error
	name   string
	path   string
}

// New binds a checklist role to its configured scheme path.
func New(name, path string) *Validator { return &Validator{name: name, path: path} }

// Validate loads the scheme once; a failed load never falls back to embedded resources.
func (v *Validator) Validate(data any) error {
	v.once.Do(func() { v.scheme, v.err = Load(v.name, v.path) })

	if v.err != nil {
		return v.err
	}

	return v.scheme.Validate(data)
}
