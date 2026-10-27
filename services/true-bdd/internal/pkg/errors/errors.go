package errors

import (
	"errors"
	"fmt"
	"strconv"
)

// Category represents the type of error.
type Category string

const (
	CategoryAI             Category = "ai"
	CategoryGitHub         Category = "github"
	CategoryParsing        Category = "parsing"
	CategoryInfrastructure Category = "infrastructure"
)

// AppError represents a structured application error.
type AppError struct {
	Category Category
	Code     string
	Message  string
	Cause    error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Category, e.Code, e.Message, e.Cause)
	}

	return fmt.Sprintf("[%s:%s] %s", e.Category, e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// AI Errors.
var (
	ErrCreateTmpDirectory = errors.New("failed to create tmp directory")
	ErrLoadData           = errors.New("failed to load data")
	ErrGenerateContent    = errors.New("failed to generate content")
	ErrWriteResponseFile  = errors.New("failed to write response file")
	ErrParseResponse      = errors.New("failed to parse response")
	ErrValidation         = errors.New("validation failed")
	ErrFileNotFound       = errors.New("file not found")
	ErrParseYAML          = errors.New("failed to parse YAML")
	ErrKeyNotFoundInYAML  = errors.New("key not found in YAML")
	ErrReadTemplateFile   = errors.New("failed to read template file")
	ErrParseTemplate      = errors.New("failed to parse template")
	ErrSendQuery          = errors.New("failed to send query")
	ErrResponseTooLarge   = errors.New("claude response too large for buffer")
)

func ErrReadTemplateFileFailed(filePath string, cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "READ_TEMPLATE_FILE_FAILED",
		Message:  "failed to read template file " + filePath,
		Cause:    errors.Join(ErrReadTemplateFile, cause),
	}
}

func ErrParseTemplateFailed(cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "PARSE_TEMPLATE_FAILED",
		Message:  "failed to parse template",
		Cause:    errors.Join(ErrParseTemplate, cause),
	}
}

func ErrResponseTooLargeForBuffer(cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "RESPONSE_TOO_LARGE",
		Message:  "Claude response too large for buffer (using streaming approach)",
		Cause:    errors.Join(ErrResponseTooLarge, cause),
	}
}

func ErrClaudeExecutionFailed(cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "CLAUDE_EXECUTION_FAILED",
		Message:  "claude execution failed",
		Cause:    errors.Join(ErrGenerateContent, cause),
	}
}

// Infrastructure Errors.
var (
	ErrDocumentNotConfigured      = errors.New("document path not configured")
	ErrLoadDocument               = errors.New("failed to load document")
	ErrLoadTemplateFile           = errors.New("failed to load template file")
	ErrInitializeConfig           = errors.New("failed to initialize config")
	ErrCreateAIClient             = errors.New("failed to create AI client")
	ErrFindStoryFile              = errors.New("failed to find story file")
	ErrStoryFileNotFound          = errors.New("story file not found")
	ErrMultipleStoryFiles         = errors.New("multiple story files found")
	ErrReadStoryFile              = errors.New("failed to read story file")
	ErrParseStoryYAML             = errors.New("failed to parse story YAML")
	ErrStoryIDMismatch            = errors.New("story file declares another story's id")
	ErrArchUpdateNoContent        = errors.New("no content found in architecture update response")
	ErrModifierMustHaveOneKey     = errors.New("modifier must have exactly one key")
	ErrInvalidStepStatementFormat = errors.New("invalid step statement format")
)

func ErrLoadTemplateFileFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "LOAD_TEMPLATE_FILE_FAILED",
		Message:  "failed to load template file",
		Cause:    errors.Join(ErrLoadTemplateFile, cause),
	}
}

// Parsing Errors.
var (
	ErrExecuteTemplate = errors.New("failed to execute template")
)

func ErrExecuteTemplateFailed(cause error) error {
	return &AppError{
		Category: CategoryParsing,
		Code:     "EXECUTE_TEMPLATE_FAILED",
		Message:  "failed to execute template",
		Cause:    errors.Join(ErrExecuteTemplate, cause),
	}
}

// Validation Errors.
var (
	ErrScenarioValidation   = errors.New("scenario validation failed")
	ErrEmptyScenarioID      = errors.New("scenario ID cannot be empty")
	ErrNoCriteria           = errors.New("scenario must reference at least one acceptance criterion")
	ErrNoSteps              = errors.New("scenario must have at least one step")
	ErrNoKeywordSet         = errors.New("step must have at least one keyword set")
	ErrMultipleKeywords     = errors.New("step must have exactly one keyword set")
	ErrNoGivenStep          = errors.New("scenario must have at least one 'Given' step")
	ErrNoWhenStep           = errors.New("scenario must have at least one 'When' step")
	ErrNoThenStep           = errors.New("scenario must have at least one 'Then' step")
	ErrNoExamples           = errors.New("scenario outline must have at least one example")
	ErrInvalidPriority      = errors.New("priority must be P0, P1, P2, or P3")
	ErrUncoveredCriterion   = errors.New("acceptance criterion is not covered by any test scenario")
	ErrNoStatements         = errors.New("step must have at least one statement")
	ErrEmptyStatement       = errors.New("statement cannot be empty")
	ErrInvalidFirstStmt     = errors.New("first statement must be main")
	ErrInvalidFollowingStmt = errors.New("following statement must be 'and' or 'but'")
	ErrInvalidModifier      = errors.New("invalid modifier type")
	ErrEmptyCoverage        = errors.New("coverage value cannot be empty")
	ErrInvalidCoverage      = errors.New("coverage value should be a percentage")
)

// Filesystem Errors.
var (
	ErrCreateDirectory = errors.New("failed to create directory")
	ErrReadConfig      = errors.New("failed to read config")
)

func ErrCreateRunDirectoryFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "CREATE_RUN_DIRECTORY_FAILED",
		Message:  "failed to create run directory",
		Cause:    errors.Join(ErrCreateDirectory, cause),
	}
}

func ErrReadConfigFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "READ_CONFIG_FAILED",
		Message:  "failed to read config file",
		Cause:    errors.Join(ErrReadConfig, cause),
	}
}

// YAML Errors.
var (
	ErrMarshalYAML   = errors.New("failed to marshal to YAML")
	ErrUnmarshalYAML = errors.New("failed to unmarshal from YAML")
)

var (
	ErrClaudeStreamNoOutput = errors.New("claude process produced no output")
)

func ErrInitializeConfigFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "INITIALIZE_CONFIG_FAILED",
		Message:  "failed to initialize config",
		Cause:    errors.Join(ErrInitializeConfig, cause),
	}
}

func ErrCreateAIClientFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "CREATE_AI_CLIENT_FAILED",
		Message:  "failed to create AI client",
		Cause:    errors.Join(ErrCreateAIClient, cause),
	}
}

func ErrFindStoryFileFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "FIND_STORY_FILE_FAILED",
		Message:  "failed to find story file",
		Cause:    errors.Join(ErrFindStoryFile, cause),
	}
}

func ErrStoryFileNotFoundError(storyID, storiesDir, format string) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "STORY_FILE_NOT_FOUND",
		Message: fmt.Sprintf(
			"no story file found for story %s in %s (expected format: %s-<slug>.yaml)",
			storyID,
			storiesDir,
			format,
		),
		Cause: ErrStoryFileNotFound,
	}
}

func ErrMultipleStoryFilesError(storyID string, matches []string) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "MULTIPLE_STORY_FILES",
		Message:  fmt.Sprintf("multiple story files found for story %s: %v", storyID, matches),
		Cause:    ErrMultipleStoryFiles,
	}
}

func ErrReadStoryFileFailed(storyFile string, cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "READ_STORY_FILE_FAILED",
		Message:  "failed to read story file " + storyFile,
		Cause:    errors.Join(ErrReadStoryFile, cause),
	}
}

func ErrParseStoryYAMLFailed(storyFile string, cause error) error {
	return &AppError{
		Category: CategoryParsing,
		Code:     "PARSE_STORY_YAML_FAILED",
		Message:  "failed to parse story YAML from " + storyFile,
		Cause:    errors.Join(ErrParseStoryYAML, cause),
	}
}

// ErrStoryIDMismatchError names a story file the `<id>-*.yaml` glob found
// for storyID whose own `id:` is another Story's: `scen-*.yaml` also
// matches `scen-check-x.yaml`, so the glob alone cannot tell them apart.
func ErrStoryIDMismatchError(storyID, storyFile, declaredID string) error {
	declared := "id " + declaredID
	if declaredID == "" {
		declared = "no id"
	}

	return &AppError{
		Category: CategoryParsing,
		Code:     "STORY_ID_MISMATCH",
		Message:  fmt.Sprintf("story file %s declares %s, not %s", storyFile, declared, storyID),
		Cause:    ErrStoryIDMismatch,
	}
}

func ErrInvalidModifierError(modifierType string) error {
	return &AppError{
		Category: CategoryParsing,
		Code:     "INVALID_MODIFIER",
		Message:  fmt.Sprintf("invalid modifier type: %s (must be 'and' or 'but')", modifierType),
		Cause:    ErrInvalidModifier,
	}
}

// Checklist Validation Errors.
var (
	ErrLoadChecklistSystemPrompt   = errors.New("failed to load checklist system prompt")
	ErrLoadChecklistUserPrompt     = errors.New("failed to load checklist user prompt")
	ErrChecklistAIEvaluation       = errors.New("AI evaluation failed")
	ErrFixApplierNoContent         = errors.New("no FILE_START/FILE_END content found")
	ErrChecklistAnswerMissing      = errors.New("checklist answer carried no FILE_START/FILE_END block")
	ErrChecklistAnswerUnparseable  = errors.New("checklist answer did not decode")
	ErrChecklistAnswerNonCanonical = errors.New("checklist answer was neither pass nor fail")
	ErrFixGeneratorNoContent       = errors.New("fix generator answer carried neither questions nor a fix prompt")
	ErrFixGeneratorBadQuestions    = errors.New("fix generator questions did not decode")
	ErrResultSchemaUnsupported     = errors.New("this cli cannot enforce a result schema")
	ErrFixNotApplied               = errors.New("fix applier reported the fix was not applied")
	ErrFixLoopStuck                = errors.New("fix loop is not converging")
	ErrFixPromptGeneration         = errors.New("fix prompt generation failed")
	ErrSaveStoryVersion            = errors.New("failed to save story version")
	ErrWriteStoryFile              = errors.New("failed to write story file")
)

func ErrLoadChecklistSystemPromptFailed(cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "LOAD_CHECKLIST_SYSTEM_PROMPT_FAILED",
		Message:  "failed to load checklist system prompt template",
		Cause:    errors.Join(ErrLoadChecklistSystemPrompt, cause),
	}
}

func ErrLoadChecklistUserPromptFailed(cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "LOAD_CHECKLIST_USER_PROMPT_FAILED",
		Message:  "failed to load checklist user prompt template",
		Cause:    errors.Join(ErrLoadChecklistUserPrompt, cause),
	}
}

func ErrChecklistAIEvaluationFailed(cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "CHECKLIST_AI_EVALUATION_FAILED",
		Message:  "AI evaluation of checklist prompt failed",
		Cause:    errors.Join(ErrChecklistAIEvaluation, cause),
	}
}

// ErrChecklistAnswerMissingBlock marks a model answer with no result block.
// A missing answer is an infrastructure failure, never a graded fail.
func ErrChecklistAnswerMissingBlock(resultPath string) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "CHECKLIST_ANSWER_MISSING",
		Message:  "no FILE_START/FILE_END content found for path: " + resultPath,
		Cause:    ErrChecklistAnswerMissing,
	}
}

// ErrChecklistAnswerDidNotParse marks a result block that is not valid YAML.
func ErrChecklistAnswerDidNotParse(resultPath string, cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "CHECKLIST_ANSWER_UNPARSEABLE",
		Message:  "result content did not decode for path: " + resultPath,
		Cause:    errors.Join(ErrChecklistAnswerUnparseable, cause),
	}
}

// ErrChecklistAnswerNotCanonical marks an answer outside the universal
// pass/fail contract. Grading it silently is how a fabricated verdict enters.
func ErrChecklistAnswerNotCanonical(resultPath, answer string) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "CHECKLIST_ANSWER_NON_CANONICAL",
		Message:  "answer " + strconv.Quote(answer) + " is neither pass nor fail for path: " + resultPath,
		Cause:    ErrChecklistAnswerNonCanonical,
	}
}

// ErrFixGeneratorEmptyAnswer marks a fix-generator answer carrying neither
// questions nor a fix prompt — distinct from a checklist with no `F:` cell.
func ErrFixGeneratorEmptyAnswer(resultPath string) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "FIX_GENERATOR_NO_CONTENT",
		Message:  "no questions and no fix prompt found for path: " + resultPath,
		Cause:    ErrFixGeneratorNoContent,
	}
}

// ErrFixGeneratorQuestionsDidNotParse marks a mangled QUESTIONS block.
func ErrFixGeneratorQuestionsDidNotParse(cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "FIX_GENERATOR_BAD_QUESTIONS",
		Message:  "questions block did not decode",
		Cause:    errors.Join(ErrFixGeneratorBadQuestions, cause),
	}
}

func ErrFixApplierNoContentFound(resultPath string) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "FIX_APPLIER_NO_CONTENT",
		Message:  "no FILE_START/FILE_END content found for path: " + resultPath,
		Cause:    ErrFixApplierNoContent,
	}
}

// ErrFixNotAppliedByModel reports an applier turn that ran to completion
// but wrote nothing, self-reported via `applied: false`. A hard stop, not a
// retry: the blocker (a denied write root, a forbidden path) is environmental and an identical retry would repeat it.
func ErrFixNotAppliedByModel(target, summary string) error {
	message := "fix applier reported applied: false"
	if target != "" {
		message += " for " + target
	}

	if summary != "" {
		message += ": " + summary
	}

	return &AppError{
		Category: CategoryAI,
		Code:     "FIX_NOT_APPLIED",
		Message:  message,
		Cause:    ErrFixNotApplied,
	}
}

// ErrFixLoopNotConverging reports a cell whose fixes kept reporting success
// without ever making the check pass. Without this the walk restarts on
// every applied fix and never terminates — a host project has no external timeout to stop it.
func ErrFixLoopNotConverging(applies int) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "FIX_LOOP_NOT_CONVERGING",
		Message: fmt.Sprintf(
			"gave up after %d applied fix(es) that never made the check pass",
			applies,
		),
		Cause: ErrFixLoopStuck,
	}
}

func ErrFixPromptGenerationFailed(cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "FIX_PROMPT_GENERATION_FAILED",
		Message:  "fix prompt generation failed",
		Cause:    errors.Join(ErrFixPromptGeneration, cause),
	}
}

func ErrSaveStoryVersionFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "SAVE_STORY_VERSION_FAILED",
		Message:  "failed to save story version",
		Cause:    errors.Join(ErrSaveStoryVersion, cause),
	}
}

func ErrWriteStoryFileFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "WRITE_STORY_FILE_FAILED",
		Message:  "failed to write story file",
		Cause:    errors.Join(ErrWriteStoryFile, cause),
	}
}

// Multi-provider routing errors: an unresolvable tier or an unregistered
// CLI is always fatal — the engine must never silently fall back to a
// different model than the checklist asked for.
var (
	ErrProviderNotRegistered = errors.New("no provider registered for cli")
	ErrProviderExecution     = errors.New("provider execution failed")
	ErrProviderNoOutput      = errors.New("provider produced no output")
	ErrResolveModelTier      = errors.New("failed to resolve model tier")
)

func ErrProviderNotRegisteredForCLI(cli string) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "PROVIDER_NOT_REGISTERED",
		Message:  "no provider registered for cli " + cli,
		Cause:    ErrProviderNotRegistered,
	}
}

func ErrProviderExecutionFailed(name string, cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "PROVIDER_EXECUTION_FAILED",
		Message:  name + " execution failed",
		Cause:    errors.Join(ErrProviderExecution, cause),
	}
}

func ErrProviderProducedNoOutput(name string) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "PROVIDER_NO_OUTPUT",
		Message:  name + " produced no output",
		Cause:    ErrProviderNoOutput,
	}
}

func ErrResolveModelTierFailed(role string, cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "RESOLVE_MODEL_TIER_FAILED",
		Message:  "failed to resolve model tier for " + role,
		Cause:    errors.Join(ErrResolveModelTier, cause),
	}
}

var ErrWriteProviderConfig = errors.New("failed to write generated provider config")

func ErrWriteProviderConfigFailed(path string, cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "WRITE_PROVIDER_CONFIG_FAILED",
		Message:  "failed to write generated provider config " + path,
		Cause:    errors.Join(ErrWriteProviderConfig, cause),
	}
}

var ErrInvalidModelConfig = errors.New("invalid engine model configuration")

func ErrInvalidModelConfigFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "INVALID_MODEL_CONFIG",
		Message:  "invalid engine model configuration in true-bdd.yaml",
		Cause:    cause,
	}
}
