package testing

import "testing"

func TestParseMyIPWTFJSON(t *testing.T) {
	body := `{
		"YourFuckingIPAddress":"185.220.101.1",
		"YourFuckingLocation":"Stockholm, Sweden",
		"YourFuckingHostname":"example.exit.node",
		"YourFuckingISP":"Example ISP",
		"YourFuckingCity":"Stockholm",
		"YourFuckingCountry":"Sweden",
		"YourFuckingCountryCode":"SE"
	}`

	got, err := parseMyIPWTFJSON(body)
	if err != nil {
		t.Fatalf("parseMyIPWTFJSON: %v", err)
	}
	if got.IP != "185.220.101.1" {
		t.Fatalf("IP = %q", got.IP)
	}
	if got.Location != "Stockholm, Sweden" {
		t.Fatalf("Location = %q", got.Location)
	}
	if got.Hostname != "example.exit.node" {
		t.Fatalf("Hostname = %q", got.Hostname)
	}
	if got.ISP != "Example ISP" {
		t.Fatalf("ISP = %q", got.ISP)
	}
	if got.City != "Stockholm" || got.Country != "Sweden" || got.CountryCode != "SE" {
		t.Fatalf("unexpected geo payload: %+v", got)
	}
}

func TestParseIPProbeBody_PlainTextFallback(t *testing.T) {
	got, err := parseIPProbeBody("185.220.101.1\n", "https://example.com/ip")
	if err != nil {
		t.Fatalf("parseIPProbeBody: %v", err)
	}
	if got.IP != "185.220.101.1" {
		t.Fatalf("IP = %q", got.IP)
	}
	if got.Location != "" || got.ISP != "" {
		t.Fatalf("expected empty geo fields, got %+v", got)
	}
}

func TestParseIPProbeBody_InvalidResponse(t *testing.T) {
	if _, err := parseIPProbeBody(`{"hello":"world"}`, "https://example.com/ip"); err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
	if _, err := parseIPProbeBody(`not-an-ip`, "https://example.com/ip"); err == nil {
		t.Fatal("expected error for invalid plain response")
	}
}

func TestFormatIPGeoSummary(t *testing.T) {
	got := FormatIPGeoSummary(&IPGeoInfo{
		City:        "Stockholm",
		CountryCode: "SE",
		ISP:         "Example ISP",
	})
	want := "Stockholm, SE, ISP: Example ISP"
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}
