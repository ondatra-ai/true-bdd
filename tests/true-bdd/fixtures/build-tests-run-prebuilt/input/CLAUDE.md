# Prebuilt engine fixture

Only edit tests/calc/steps/. Register and implement the missing parameterized "the help lists (.+)" assertion: State.Help must contain the captured command name, otherwise return an error. Reuse the existing State and definitions. The existing When definition requests --help from State.EngineBin. Do not edit the runner, scenario shim, Registry or architecture. There is deliberately no services/calc source. The replay entry point consumes -engine-bin and skips building the service when supplied.
