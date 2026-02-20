//go:build ast

package ast

// buildExtractorRegistry returns a file-extension-to-extractor map
// for the configured set of enabled language names.
// Full build: Go (go/parser) + TypeScript + Python + JavaScript (tree-sitter).
func buildExtractorRegistry(enabledLanguages []string) map[string]LanguageExtractor {
	available := []LanguageExtractor{
		&GoExtractor{},
		&TypeScriptExtractor{},
		&PythonExtractor{},
		&JavaScriptExtractor{},
	}

	enabled := make(map[string]bool, len(enabledLanguages))
	for _, lang := range enabledLanguages {
		enabled[lang] = true
	}

	registry := make(map[string]LanguageExtractor)
	for _, extractor := range available {
		if !enabled[extractor.Language()] {
			continue
		}
		for _, ext := range extractor.Extensions() {
			registry[ext] = extractor
		}
	}
	return registry
}
