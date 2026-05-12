package docs

// ArchitectureDoc represents an architecture document with content and file path.
type ArchitectureDoc struct {
	Content  string
	FilePath string
}

// ArchitectureDocs represents all loaded architecture documents.
type ArchitectureDocs struct {
	Architecture         ArchitectureDoc
	FrontendArchitecture ArchitectureDoc
	CodingStandards      ArchitectureDoc
	SourceTree           ArchitectureDoc
	TechStack            ArchitectureDoc
}
