package ignore

// DefaultIgnorePatterns contains patterns that should always be ignored during indexing.
// These are common directories and files that are never useful for code search.
var DefaultIgnorePatterns = []string{
	// Version control
	".git",
	".svn",
	".hg",

	// Dependencies
	"node_modules",
	"vendor",
	"bower_components",
	".npm",
	".yarn",
	".pnp.*",

	// Build output
	"dist",
	"build",
	"out",
	"target",
	"bin",
	"obj",

	// IDE / Editor
	".idea",
	".vscode",
	".vs",
	"*.swp",
	"*.swo",
	"*~",

	// OS files
	".DS_Store",
	"Thumbs.db",
	"desktop.ini",

	// Python
	"__pycache__",
	"*.pyc",
	"*.pyo",
	".venv",
	"venv",
	".env",

	// Go
	".go",
	// (vendor already listed)

	// Compiled / Binary extensions
	"*.exe",
	"*.dll",
	"*.so",
	"*.dylib",
	"*.o",
	"*.a",
	"*.lib",
	"*.class",
	"*.jar",
	"*.war",

	// Archives
	"*.zip",
	"*.tar",
	"*.tar.gz",
	"*.tgz",
	"*.rar",
	"*.7z",

	// Images
	"*.png",
	"*.jpg",
	"*.jpeg",
	"*.gif",
	"*.bmp",
	"*.ico",
	"*.webp",
	"*.tiff",

	// Fonts
	"*.woff",
	"*.woff2",
	"*.ttf",
	"*.eot",
	"*.otf",

	// Media
	"*.mp3",
	"*.mp4",
	"*.avi",
	"*.mov",
	"*.wav",
	"*.flac",

	// Documents
	"*.pdf",
	"*.doc",
	"*.docx",
	"*.xls",
	"*.xlsx",
	"*.ppt",
	"*.pptx",

	// Minified files
	"*.min.js",
	"*.min.css",

	// Lock files
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"Gemfile.lock",
	"poetry.lock",
	"Cargo.lock",
	"go.sum",
	"composer.lock",

	// Source maps
	"*.map",

	// Coverage
	"coverage",
	".nyc_output",
	"htmlcov",

	// Cache
	".cache",
	".parcel-cache",
	".next",
	".nuxt",

	// Logs
	"*.log",

	// Database files
	"*.sqlite",
	"*.sqlite3",
	"*.db",
}
