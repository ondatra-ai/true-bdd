package story

type DevAgentRecord struct {
	AgentModelUsed     *string  `json:"agent_model_used"     yaml:"agent_model_used"`
	DebugLogReferences []string `json:"debug_log_references" yaml:"debug_log_references"`
	CompletionNotes    []string `json:"completion_notes"     yaml:"completion_notes"`
	FileList           []string `json:"file_list"            yaml:"file_list"`
}
