package baseline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

func TestVersionFourRoundTripAndDistinctIdentities(t *testing.T) {
	issues := []analyzer.Issue{
		{Check: analyzer.CheckSRPDataClump, Subject: "p.First", Identity: "parameters=a,b"},
		{Check: analyzer.CheckSRPDataClump, Subject: "p.Second", Identity: "parameters=a,b"},
	}
	for index := range issues {
		issues[index].Pos.Filename = "p.go"
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := Write(path, issues); err != nil {
		t.Fatal(err)
	}
	accepted, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(accepted) != 2 || len(Filter(append([]analyzer.Issue(nil), issues...), accepted)) != 0 {
		t.Fatalf("round trip accepted %d fingerprints", len(accepted))
	}
}

func TestReadRejectsLegacyVersionAndMalformedHash(t *testing.T) {
	for _, body := range []string{
		`{"version":3,"fingerprints":[]}`,
		`{"version":4,"fingerprints":["ABC"]}`,
	} {
		path := filepath.Join(t.TempDir(), "baseline.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(path); err == nil || (!strings.Contains(err.Error(), "regenerate") && !strings.Contains(err.Error(), "64-character")) {
			t.Fatalf("Read(%s) error = %v", body, err)
		}
	}
}
