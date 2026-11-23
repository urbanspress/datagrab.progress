package main

import "testing"

func TestBrandOrganizationReadsOnlyCanonicalName(t *testing.T) {
	const profile = "# Brand DNA\r\n\r\n## Profile Data\r\n\r\n```json\r\n"
	valid := profile + `{"schema":"brand-dna.v1","business_name":" Édition & Co. ","password":"never shared"}` + "\r\n```\r\n"
	if got, err := brandOrganization(valid); got != "Édition & Co." || err != nil {
		t.Fatalf("unexpected organization %q: %v", got, err)
	}
	for _, input := range []string{
		"# Brand DNA\nFake Business",
		profile + `{"schema":"brand-dna.v2","business_name":"Example"}` + "\n```",
		profile + `{"schema":"brand-dna.v1","business_name":"hello\nworld"}` + "\n```",
	} {
		if _, err := brandOrganization(input); err == nil {
			t.Fatalf("invalid brand profile was accepted: %q", input)
		}
	}
}

func TestBrowserPaneExactOriginAndWorkbenchMessages(t *testing.T) {
	origins := parseBrowserOrigins("https://example.com,https://example.org:8443,http://bad.example,https://path.example/admin")
	for _, website := range []string{"https://example.com/books?q=test", "https://example.com:443/books", "https://example.org:8443"} {
		if origins.match(website) == "" {
			t.Errorf("approved URL rejected: %s", website)
		}
	}
	for _, website := range []string{
		"http://example.com", "https://example.com:8443", "https://www.example.com", "https://example.org",
		"https://example.com@wrong.org/", "https://foo@example.com/", "https://example.com./", "https://path.example/",
		"about:blank", "file:///C:/Windows/system.ini", "javascript:void(0)", "https://127.0.0.1:9490/",
	} {
		if origins.match(website) != "" {
			t.Errorf("unapproved URL allowed: %s", website)
		}
	}
	const base = "http://127.0.0.1:9490"
	for _, source := range []string{
		base,
		base + "/api",
		base + "/?x=1",
		base + "/?x=1#studio=code",
		base + "/#evil",
		base + "/#studio=other",
		"http://user@127.0.0.1:9490/#studio=code",
		"http://127.0.0.1:9491/",
		"https://example.com/",
	} {
		if trustedWorkbenchSource(source, base) {
			t.Errorf("browser source allowed to send commands: %s", source)
		}
	}
	for _, allowedSource := range []string{
		base + "/",
		base + "/#studio=database",
		base + "/#studio=browser",
		base + "/#studio=code",
		base + "/#desk-workspace-stage",
	} {
		if !trustedWorkbenchSource(allowedSource, base) {
			t.Fatalf("workbench origin rejected: %s", allowedSource)
		}
	}
	for _, message := range []string{
		`{"type":"groxy-browser-open","url":"https://example.com","script":"alert(1)"}`,
		`{"type":"groxy-desk-ready","url":"https://example.com"}`,
		`{"type":"groxy-browser-capture","url":"https://example.com"}`,
		`{"type":"groxy-browser-resize","width":100}`,
		`{"type":"groxy-browser-resize","width":"30"}`,
		`{"type":"groxy-browser-close"} {"type":"groxy-desk-ready"}`,
		`{"type":"execute-script"}`,
		`{"type":"win_close","url":""}`,
		`{"type":"win_minimize","width":0}`,
		`{"type":"win_toggle_maximize","handle":1}`,
	} {
		if _, ok := decodePaneCommand(message); ok {
			t.Errorf("unsupported message accepted: %s", message)
		}
	}
	if _, ok := decodePaneCommand(`{"type":"groxy-browser-resize","width":45}`); !ok {
		t.Fatal("valid resize rejected")
	}
	if _, ok := decodePaneCommand(`{"type":"groxy-browser-capture"}`); !ok {
		t.Fatal("fixed selection capture rejected")
	}
	for _, action := range []string{"win_minimize", "win_toggle_maximize", "win_close"} {
		if command, ok := decodePaneCommand(`{"type":"` + action + `"}`); !ok || command.Type != action {
			t.Errorf("valid window action rejected: %s", action)
		}
	}
}
