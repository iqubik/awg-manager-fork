package dnsroute

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreLoadBackfillsRawEditorText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dns-routes.json")

	raw := []byte(`{
  "lists": [
    {
      "id": "list_1",
      "name": "legacy",
      "domains": ["youtube.com"],
      "manualDomains": ["youtube.com", "googlevideo.com"],
      "excludes": ["local"],
      "excludeSubnets": ["10.0.0.0/8"],
      "routes": []
    }
  ]
}`)
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dir)
	data, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Lists) != 1 {
		t.Fatalf("lists len = %d, want 1", len(data.Lists))
	}

	list := data.Lists[0]

	if list.ManualText == nil {
		t.Fatal("ManualText was not backfilled")
	}
	if *list.ManualText != "youtube.com\ngooglevideo.com" {
		t.Fatalf("ManualText = %q", *list.ManualText)
	}

	if list.ExcludesText == nil {
		t.Fatal("ExcludesText was not backfilled")
	}
	if *list.ExcludesText != "local\n10.0.0.0/8" {
		t.Fatalf("ExcludesText = %q", *list.ExcludesText)
	}
}
