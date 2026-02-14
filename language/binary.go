package language

// IsBinaryContent checks if the given byte slice appears to be binary content.
// It checks the first 512 bytes (or less) for null bytes, which indicates binary data.
func IsBinaryContent(data []byte) bool {
	checkSize := 512
	if len(data) < checkSize {
		checkSize = len(data)
	}

	for i := 0; i < checkSize; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}
