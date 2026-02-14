package language

import "testing"

func Test_DetectLanguage_GoFile(t *testing.T) {
	lang := DetectLanguage("main.go")
	if lang != "Go" {
		t.Errorf("expected Go, got %s", lang)
	}
}

func Test_DetectLanguage_TypeScriptFile(t *testing.T) {
	lang := DetectLanguage("src/components/App.tsx")
	if lang != "TypeScript" {
		t.Errorf("expected TypeScript, got %s", lang)
	}
}

func Test_DetectLanguage_Makefile(t *testing.T) {
	lang := DetectLanguage("Makefile")
	if lang != "Makefile" {
		t.Errorf("expected Makefile, got %s", lang)
	}
}

func Test_DetectLanguage_UnknownExtension(t *testing.T) {
	lang := DetectLanguage("data.xyz")
	if lang != "Unknown" {
		t.Errorf("expected Unknown, got %s", lang)
	}
}

func Test_DetectLanguage_CaseInsensitive(t *testing.T) {
	lang := DetectLanguage("README.MD")
	if lang != "Markdown" {
		t.Errorf("expected Markdown, got %s", lang)
	}
}
