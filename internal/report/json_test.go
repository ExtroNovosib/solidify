package report

import (
	"bytes"
	"testing"
)

func TestEncodeEmptyJSON(t *testing.T) {
	data, err := EncodeJSON(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(data), []byte("[]")) {
		t.Fatalf("JSON = %s", data)
	}
}
