package language

import (
	"path/filepath"
	"strings"
)

// ExtensionToLanguage maps file extensions (without dot) to language names.
var ExtensionToLanguage = map[string]string{
	// Go
	"go": "Go",
	// JavaScript / TypeScript
	"js": "JavaScript", "jsx": "JavaScript", "mjs": "JavaScript", "cjs": "JavaScript",
	"ts": "TypeScript", "tsx": "TypeScript", "mts": "TypeScript", "cts": "TypeScript",
	// Python
	"py": "Python", "pyi": "Python", "pyw": "Python",
	// Rust
	"rs": "Rust",
	// Java / Kotlin
	"java": "Java", "kt": "Kotlin", "kts": "Kotlin",
	// C / C++
	"c": "C", "h": "C",
	"cpp": "C++", "cc": "C++", "cxx": "C++", "hpp": "C++", "hxx": "C++",
	// C#
	"cs": "C#", "csx": "C#",
	// Swift
	"swift": "Swift",
	// Dart
	"dart": "Dart",
	// Ruby
	"rb": "Ruby", "erb": "Ruby",
	// PHP
	"php": "PHP",
	// Shell
	"sh": "Shell", "bash": "Shell", "zsh": "Shell", "fish": "Shell",
	"ps1": "PowerShell", "psm1": "PowerShell", "psd1": "PowerShell",
	// Web
	"html": "HTML", "htm": "HTML",
	"css": "CSS", "scss": "SCSS", "sass": "Sass", "less": "Less",
	// Data / Config
	"json": "JSON", "jsonc": "JSON",
	"yaml": "YAML", "yml": "YAML",
	"toml": "TOML",
	"xml": "XML", "xsl": "XML", "xslt": "XML",
	"ini": "INI",
	"env": "Env",
	"properties": "Properties",
	// Markup
	"md": "Markdown", "mdx": "Markdown",
	"rst": "reStructuredText",
	"tex": "LaTeX",
	// SQL
	"sql": "SQL",
	// GraphQL
	"graphql": "GraphQL", "gql": "GraphQL",
	// Protocol Buffers
	"proto": "Protobuf",
	// Docker
	"dockerfile": "Dockerfile",
	// Terraform
	"tf": "Terraform", "tfvars": "Terraform",
	// Lua
	"lua": "Lua",
	// R
	"r": "R", "rmd": "R",
	// Scala
	"scala": "Scala",
	// Elixir / Erlang
	"ex": "Elixir", "exs": "Elixir",
	"erl": "Erlang", "hrl": "Erlang",
	// Haskell
	"hs": "Haskell",
	// Zig
	"zig": "Zig",
	// Vue / Svelte
	"vue": "Vue", "svelte": "Svelte",
	// Misc
	"txt": "Text",
	"csv": "CSV",
	"svg": "SVG",
	"bat": "Batch", "cmd": "Batch",
	"makefile": "Makefile",
	"cmake": "CMake",
	"gradle": "Gradle",
}

// DetectLanguage returns the programming language for a file path based on its extension.
// Returns "Unknown" if the extension is not recognized.
func DetectLanguage(filePath string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
	if ext == "" {
		// Check filename-based detection (e.g., Makefile, Dockerfile)
		base := strings.ToLower(filepath.Base(filePath))
		switch base {
		case "makefile", "gnumakefile":
			return "Makefile"
		case "dockerfile":
			return "Dockerfile"
		case "cmakelists.txt":
			return "CMake"
		case "gemfile", "rakefile":
			return "Ruby"
		case ".gitignore", ".gitattributes":
			return "Git Config"
		case ".env", ".env.local", ".env.example":
			return "Env"
		}
		return "Unknown"
	}

	if lang, ok := ExtensionToLanguage[ext]; ok {
		return lang
	}
	return "Unknown"
}
