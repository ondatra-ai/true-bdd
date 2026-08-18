// Package scenariogen renders one Go test per registry scenario.
//
// The registry is the spec; these files are what runs it. They are
// deterministic output — the same registry always renders the same
// bytes — which is what lets `build tests` answer "is this repository's
// test suite the one its scenarios describe?" by regenerating and
// comparing, instead of by asking a model to read the tree.
//
// Nothing here is a judgement call, and that is the division of labour
// with the fix loop: transcribing a scenario into a test file is
// mechanical, so the engine does it; authoring the step definition that
// binds a step is not, so a model does that.
package scenariogen

import "errors"

// ErrNoPath signals a scenario with no `path:`. Required and never
// defaulted: which behaviours belong in one file is editorial, and a
// generator that guessed would scatter one command's scenarios across
// forty files.
var ErrNoPath = errors.New("scenario declares no path: for its generated test")

// ErrPathNotRelative signals a `path:` that is absolute, escapes the
// repository, or is not already in cleaned form.
var ErrPathNotRelative = errors.New("path: must be a clean repo-relative path")

// ErrPathNotTestFile signals a `path:` Go would not compile as a test.
var ErrPathNotTestFile = errors.New("path: must name a _test.go file")

// ErrPathNotInSuiteRoot signals a `path:` outside the directory the
// owning suite declares.
//
// Directly inside it, not merely underneath: a file one directory down
// is a different Go package, needing its own shim and its own
// TestMain, and a generator that silently created those would be
// inventing structure the spec never asked for.
var ErrPathNotInSuiteRoot = errors.New("path: must sit directly in the owning suite's directory")

// ErrPathInStepsTree signals a `path:` inside the suite's steps/ tree.
// That tree belongs to the fix turn; a generated file written into it
// could overwrite a step definition, and the definitions are the one
// thing here no regeneration can reproduce.
var ErrPathInStepsTree = errors.New("path: must not be inside the suite's steps/ tree")

// ErrPathCrossSuite signals two scenarios sharing a `path:` while naming
// different services. One file is one Go package bound to one suite's
// state; two suites' scenarios in it could not both compile.
var ErrPathCrossSuite = errors.New("scenarios sharing a path: must name the same service")

// ErrNoSuiteForService signals a scenario whose `service:` matches no
// `architecture.testing.suites[]` entry — a scenario no suite runs.
var ErrNoSuiteForService = errors.New("no test suite declares this scenario's service")

// ErrNoGeneratorForFramework signals a suite whose `framework:` has no
// renderer. A named refusal rather than a skip: a suite quietly left
// ungenerated is a suite whose scenarios nothing executes.
var ErrNoGeneratorForFramework = errors.New("no test generator for this suite's framework")

// ErrIdentifierCollision signals two scenario ids that render the same
// Go function name — `E2E-001` and `E2E001` both give TestE2E001, and
// Go would refuse the file that held both.
var ErrIdentifierCollision = errors.New("two scenario ids render the same Go test name")

// ErrUnrunnableTestName signals a scenario id whose Go function `go
// test` would compile and never run — the worst available failure,
// since every coverage check still reports the scenario as covered.
var ErrUnrunnableTestName = errors.New("scenario id renders a name `go test` will not run")

// ErrPackageNameInvalid signals a suite name that derives something Go
// would not accept as a package identifier.
var ErrPackageNameInvalid = errors.New("suite name derives an invalid Go package name")

// ErrPackageClauseMismatch signals a target directory already declaring
// a different package — almost always a suite renamed in the spec
// without its hand-written files following.
var ErrPackageClauseMismatch = errors.New("the target directory declares a different package")

// ErrScenarioHasNoSteps signals a scenario with nothing to run. Its
// generated test would declare a name, assert nothing, and pass.
var ErrScenarioHasNoSteps = errors.New("scenario has no steps, so its test would pass trivially")

// ErrWouldClobberHandWritten signals a target file that exists but
// carries no generated marker. A typo'd `path:` must not be able to
// delete a step definition.
var ErrWouldClobberHandWritten = errors.New("refusing to overwrite a file that is not marked generated")

// ErrNoModuleForSuite signals a suite with no go.mod above it, so the
// import path of its scenarios package cannot be computed.
var ErrNoModuleForSuite = errors.New("no go.mod above the suite's directory")
