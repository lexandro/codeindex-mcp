package language

import "testing"

func Test_IsBinaryContent_TextFile(t *testing.T) {
	content := []byte("Hello, this is a text file\nwith multiple lines\n")
	if IsBinaryContent(content) {
		t.Error("expected text content to not be detected as binary")
	}
}

func Test_IsBinaryContent_BinaryFile(t *testing.T) {
	content := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00} // PNG header with null byte
	if !IsBinaryContent(content) {
		t.Error("expected binary content to be detected as binary")
	}
}

func Test_IsBinaryContent_EmptyFile(t *testing.T) {
	content := []byte{}
	if IsBinaryContent(content) {
		t.Error("expected empty content to not be detected as binary")
	}
}

func Test_IsBinaryContent_NullInMiddle(t *testing.T) {
	content := make([]byte, 100)
	for i := range content {
		content[i] = 'a'
	}
	content[50] = 0x00
	if !IsBinaryContent(content) {
		t.Error("expected content with null byte to be detected as binary")
	}
}
