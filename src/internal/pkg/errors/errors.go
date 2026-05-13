package errors

import (
	"errors"
	"fmt"
)

// Category represents the type of error.
type Category string

const (
	CategoryAI             Category = "ai"
	CategoryGitHub         Category = "github"
	CategoryParsing        Category = "parsing"
	CategoryInfrastructure Category = "infrastructure"
	CategoryParser         Category = "parser"
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
	ErrCreateTmpDirectory  = errors.New("failed to create tmp directory")
	ErrLoadData            = errors.New("failed to load data")
	ErrGenerateContent     = errors.New("failed to generate content")
	ErrWriteResponseFile   = errors.New("failed to write response file")
	ErrParseResponse       = errors.New("failed to parse response")
	ErrValidation          = errors.New("validation failed")
	ErrFileNotFound        = errors.New("file not found")
	ErrParseYAML           = errors.New("failed to parse YAML")
	ErrKeyNotFoundInYAML   = errors.New("key not found in YAML")
	ErrReadTemplateFile    = errors.New("failed to read template file")
	ErrParseTemplate       = errors.New("failed to parse template")
	ErrSendQuery           = errors.New("failed to send query")
	ErrClaudeReturnedError = errors.New("claude returned error")
	ErrResponseTooLarge    = errors.New("claude response too large for buffer")
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

func ErrSendQueryFailed(cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "SEND_QUERY_FAILED",
		Message:  "failed to send query",
		Cause:    errors.Join(ErrSendQuery, cause),
	}
}

func ErrClaudeError(result string) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "CLAUDE_RETURNED_ERROR",
		Message:  "Claude returned error: " + result,
		Cause:    ErrClaudeReturnedError,
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
	ErrLoadStoryFromEpic          = errors.New("failed to load story from epic file")
	ErrFindStoryFile              = errors.New("failed to find story file")
	ErrStoryFileNotFound          = errors.New("story file not found")
	ErrMultipleStoryFiles         = errors.New("multiple story files found")
	ErrReadStoryFile              = errors.New("failed to read story file")
	ErrParseStoryYAML             = errors.New("failed to parse story YAML")
	ErrParseStoryNumber           = errors.New("failed to parse story number")
	ErrLoadEpic                   = errors.New("failed to load epic")
	ErrStoryIndexOutOfBounds      = errors.New("story index out of bounds")
	ErrInvalidStoryNumberFormat   = errors.New("invalid story number format")
	ErrSearchEpicFiles            = errors.New("failed to search for epic files")
	ErrNoEpicFile                 = errors.New("no epic file found")
	ErrMultipleEpicFiles          = errors.New("multiple epic files found")
	ErrReadEpicFile               = errors.New("failed to read epic file")
	ErrParseEpicYAML              = errors.New("failed to parse epic YAML")
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
	ErrInvalidOptions       = errors.New("invalid options")
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
	ErrInvalidLevel         = errors.New("level must be integration or e2e")
	ErrInvalidPriority      = errors.New("priority must be P0, P1, P2, or P3")
	ErrUncoveredCriterion   = errors.New("acceptance criterion is not covered by any test scenario")
	ErrNoStatements         = errors.New("step must have at least one statement")
	ErrEmptyStatement       = errors.New("statement cannot be empty")
	ErrInvalidFirstStmt     = errors.New("first statement must be main")
	ErrInvalidFollowingStmt = errors.New("following statement must be 'and' or 'but'")
	ErrInvalidModifier      = errors.New("invalid modifier type")
	ErrEmptyCoverage        = errors.New("coverage value cannot be empty")
	ErrInvalidCoverage      = errors.New("coverage value should be a percentage")
	ErrACMissingSteps       = errors.New("acceptance criterion has no steps")
)

// Filesystem Errors.
var (
	ErrCreateDirectory       = errors.New("failed to create directory")
	ErrCheckWorkingDirectory = errors.New("failed to check working directory")
	ErrReadConfig            = errors.New("failed to read config")
)

func ErrCreateRunDirectoryFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "CREATE_RUN_DIRECTORY_FAILED",
		Message:  "failed to create run directory",
		Cause:    errors.Join(ErrCreateDirectory, cause),
	}
}

func ErrCheckWorkingDirectoryFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "CHECK_WORKING_DIRECTORY_FAILED",
		Message:  "failed to check working directory",
		Cause:    errors.Join(ErrCheckWorkingDirectory, cause),
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

func ErrNegativeMaxThinkingTokens(value int) error {
	return &AppError{
		Category: CategoryParsing,
		Code:     "INVALID_MAX_THINKING_TOKENS",
		Message:  fmt.Sprintf("MaxThinkingTokens must be non-negative, got %d", value),
		Cause:    ErrInvalidOptions,
	}
}

func ErrNegativeMaxTurns(value int) error {
	return &AppError{
		Category: CategoryParsing,
		Code:     "INVALID_MAX_TURNS",
		Message:  fmt.Sprintf("MaxTurns must be non-negative, got %d", value),
		Cause:    ErrInvalidOptions,
	}
}

func ErrToolInBothLists(tool string) error {
	return &AppError{
		Category: CategoryParsing,
		Code:     "TOOL_CONFLICT",
		Message:  fmt.Sprintf("tool '%s' cannot be in both AllowedTools and DisallowedTools", tool),
		Cause:    ErrInvalidOptions,
	}
}

// Parser Errors (for claudecode/internal/parser).
var (
	ErrBufferOverflow    = errors.New("buffer overflow")
	ErrParseContentBlock = errors.New("failed to parse content block")
)

func ErrBufferSizeExceeded(bufferSize, maxSize int) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "BUFFER_SIZE_EXCEEDED",
		Message:  fmt.Sprintf("buffer size %d exceeds limit %d", bufferSize, maxSize),
		Cause:    ErrBufferOverflow,
	}
}

func ErrParseContentBlockFailed(index int, cause error) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "PARSE_CONTENT_BLOCK_FAILED",
		Message:  fmt.Sprintf("failed to parse content block %d", index),
		Cause:    errors.Join(ErrParseContentBlock, cause),
	}
}

// Transport Errors (for claudecode/internal/subprocess).
var (
	ErrCreateStdinPipe  = errors.New("failed to create stdin pipe")
	ErrCreateStdoutPipe = errors.New("failed to create stdout pipe")
	ErrCreateStderrFile = errors.New("failed to create stderr file")
	ErrMarshalMessage   = errors.New("failed to marshal message")
	ErrWriteMessage     = errors.New("failed to write message")
	ErrStdoutScanner    = errors.New("stdout scanner error")
)

func ErrCreateStdinPipeFailed(cause error) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "CREATE_STDIN_PIPE_FAILED",
		Message:  "failed to create stdin pipe",
		Cause:    errors.Join(ErrCreateStdinPipe, cause),
	}
}

func ErrCreateStdoutPipeFailed(cause error) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "CREATE_STDOUT_PIPE_FAILED",
		Message:  "failed to create stdout pipe",
		Cause:    errors.Join(ErrCreateStdoutPipe, cause),
	}
}

func ErrCreateStderrFileFailed(cause error) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "CREATE_STDERR_FILE_FAILED",
		Message:  "failed to create stderr file",
		Cause:    errors.Join(ErrCreateStderrFile, cause),
	}
}

func ErrMarshalMessageFailed(cause error) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "MARSHAL_MESSAGE_FAILED",
		Message:  "failed to marshal message",
		Cause:    errors.Join(ErrMarshalMessage, cause),
	}
}

func ErrWriteMessageFailed(cause error) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "WRITE_MESSAGE_FAILED",
		Message:  "failed to write message",
		Cause:    errors.Join(ErrWriteMessage, cause),
	}
}

func ErrStdoutScannerFailed(cause error) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "STDOUT_SCANNER_FAILED",
		Message:  "stdout scanner error",
		Cause:    errors.Join(ErrStdoutScanner, cause),
	}
}

// Client Errors (for claudecode).
var (
	ErrWorkingDirectoryNotExist  = errors.New("working directory does not exist")
	ErrInvalidMaxTurns           = errors.New("invalid max_turns")
	ErrInvalidPermissionMode     = errors.New("invalid permission mode")
	ErrConnectClient             = errors.New("failed to connect client")
	ErrCLINotFound               = errors.New("claude CLI not found")
	ErrConnectTransport          = errors.New("failed to connect transport")
	ErrInvalidConfiguration      = errors.New("invalid configuration")
	ErrCloseTransport            = errors.New("failed to close transport")
	ErrClientNotConnected        = errors.New("client not connected")
	ErrTransportAlreadyConnected = errors.New("transport already connected")
	ErrTransportNotConnected     = errors.New("transport not connected or stdin closed")
	ErrProcessNotRunning         = errors.New("process not running")
	ErrInterruptNotSupported     = errors.New("interrupt not supported by windows")
	ErrClaudeStreamClosed        = errors.New("claude stream closed before receiving messages")
	ErrClaudeStreamNoOutput      = errors.New("claude process produced no output")
)

func ErrWorkingDirectoryDoesNotExist(path string) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "WORKING_DIRECTORY_NOT_EXIST",
		Message:  "working directory does not exist: " + path,
		Cause:    ErrWorkingDirectoryNotExist,
	}
}

func ErrMaxTurnsMustBeNonNegative(value int) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "INVALID_MAX_TURNS",
		Message:  fmt.Sprintf("max_turns must be non-negative, got: %d", value),
		Cause:    ErrInvalidMaxTurns,
	}
}

func ErrInvalidPermissionModeValue(mode string) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "INVALID_PERMISSION_MODE",
		Message:  "invalid permission mode: " + mode,
		Cause:    ErrInvalidPermissionMode,
	}
}

func ErrConnectClientFailed(cause error) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "CONNECT_CLIENT_FAILED",
		Message:  "failed to connect client",
		Cause:    errors.Join(ErrConnectClient, cause),
	}
}

func ErrCLINotFoundError(cause error) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "CLI_NOT_FOUND",
		Message:  "claude CLI not found",
		Cause:    errors.Join(ErrCLINotFound, cause),
	}
}

func ErrConnectTransportFailed(cause error) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "CONNECT_TRANSPORT_FAILED",
		Message:  "failed to connect transport",
		Cause:    errors.Join(ErrConnectTransport, cause),
	}
}

func ErrInvalidConfigurationError(cause error) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "INVALID_CONFIGURATION",
		Message:  "invalid configuration",
		Cause:    errors.Join(ErrInvalidConfiguration, cause),
	}
}

func ErrCloseTransportFailed(cause error) error {
	return &AppError{
		Category: CategoryParser,
		Code:     "CLOSE_TRANSPORT_FAILED",
		Message:  "failed to close transport",
		Cause:    errors.Join(ErrCloseTransport, cause),
	}
}

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

func ErrLoadStoryFromEpicFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "LOAD_STORY_FROM_EPIC_FAILED",
		Message:  "failed to load story from epic file",
		Cause:    errors.Join(ErrLoadStoryFromEpic, cause),
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

func ErrStoryFileNotFoundError(storyNumber, storiesDir, format string) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "STORY_FILE_NOT_FOUND",
		Message: fmt.Sprintf(
			"no story file found for story %s in %s (expected format: %s-<slug>.yaml)",
			storyNumber,
			storiesDir,
			format,
		),
		Cause: ErrStoryFileNotFound,
	}
}

func ErrMultipleStoryFilesError(storyNumber string, matches []string) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "MULTIPLE_STORY_FILES",
		Message:  fmt.Sprintf("multiple story files found for story %s: %v", storyNumber, matches),
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

func ErrParseStoryNumberFailed(storyNumber string, cause error) error {
	return &AppError{
		Category: CategoryParsing,
		Code:     "PARSE_STORY_NUMBER_FAILED",
		Message:  "failed to parse story number " + storyNumber,
		Cause:    errors.Join(ErrParseStoryNumber, cause),
	}
}

func ErrLoadEpicFailed(epicNum int, cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "LOAD_EPIC_FAILED",
		Message:  fmt.Sprintf("failed to load epic %d", epicNum),
		Cause:    errors.Join(ErrLoadEpic, cause),
	}
}

func ErrStoryIndexOutOfBoundsError(storyIndex, epicNum, totalStories int) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "STORY_INDEX_OUT_OF_BOUNDS",
		Message:  fmt.Sprintf("story index %d out of bounds for epic %d (has %d stories)", storyIndex, epicNum, totalStories),
		Cause:    ErrStoryIndexOutOfBounds,
	}
}

func ErrInvalidStoryNumberFormatError(storyNumber string) error {
	return &AppError{
		Category: CategoryParsing,
		Code:     "INVALID_STORY_NUMBER_FORMAT",
		Message:  "invalid story number format, expected X.Y but got " + storyNumber,
		Cause:    ErrInvalidStoryNumberFormat,
	}
}

func ErrSearchEpicFilesFailed(cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "SEARCH_EPIC_FILES_FAILED",
		Message:  "failed to search for epic files",
		Cause:    errors.Join(ErrSearchEpicFiles, cause),
	}
}

func ErrNoEpicFileError(pattern string) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "NO_EPIC_FILE",
		Message:  "no epic file found matching pattern " + pattern,
		Cause:    ErrNoEpicFile,
	}
}

func ErrMultipleEpicFilesError(pattern string, matches []string) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "MULTIPLE_EPIC_FILES",
		Message:  fmt.Sprintf("multiple epic files found matching pattern %s: %v", pattern, matches),
		Cause:    ErrMultipleEpicFiles,
	}
}

func ErrReadEpicFileFailed(epicFilePath string, cause error) error {
	return &AppError{
		Category: CategoryInfrastructure,
		Code:     "READ_EPIC_FILE_FAILED",
		Message:  "failed to read epic file " + epicFilePath,
		Cause:    errors.Join(ErrReadEpicFile, cause),
	}
}

func ErrParseEpicYAMLFailed(epicFilePath string, cause error) error {
	return &AppError{
		Category: CategoryParsing,
		Code:     "PARSE_EPIC_YAML_FAILED",
		Message:  "failed to parse epic YAML from " + epicFilePath,
		Cause:    errors.Join(ErrParseEpicYAML, cause),
	}
}

func ErrACMissingStepsError(acID string) error {
	return &AppError{
		Category: CategoryParsing,
		Code:     "AC_MISSING_STEPS",
		Message:  fmt.Sprintf("AC %s has no steps", acID),
		Cause:    ErrACMissingSteps,
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
	ErrLoadChecklistSystemPrompt = errors.New("failed to load checklist system prompt")
	ErrLoadChecklistUserPrompt   = errors.New("failed to load checklist user prompt")
	ErrChecklistAIEvaluation     = errors.New("AI evaluation failed")
	ErrFixApplierNoContent       = errors.New("no FILE_START/FILE_END content found")
	ErrFixPromptGeneration       = errors.New("fix prompt generation failed")
	ErrFixPromptRefinement       = errors.New("fix prompt refinement failed")
	ErrSaveStoryVersion          = errors.New("failed to save story version")
	ErrWriteStoryFile            = errors.New("failed to write story file")
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

func ErrFixApplierNoContentFound(resultPath string) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "FIX_APPLIER_NO_CONTENT",
		Message:  "no FILE_START/FILE_END content found for path: " + resultPath,
		Cause:    ErrFixApplierNoContent,
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

func ErrFixPromptRefinementFailed(cause error) error {
	return &AppError{
		Category: CategoryAI,
		Code:     "FIX_PROMPT_REFINEMENT_FAILED",
		Message:  "fix prompt refinement failed",
		Cause:    errors.Join(ErrFixPromptRefinement, cause),
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
