package runner

func validateCommandTemplates[I any](spec Spec[I]) error {
	err := spec.Evaluator.ValidateTemplates()
	if err != nil {
		return RefuseStartup(spec.Name, err)
	}

	if !spec.Fix {
		return nil
	}

	err = spec.FixGenerator.ValidateTemplates()
	if err != nil {
		return RefuseStartup(spec.Name, err)
	}

	err = spec.FixApplier.ValidateTemplates()
	if err != nil {
		return RefuseStartup(spec.Name, err)
	}

	return nil
}
