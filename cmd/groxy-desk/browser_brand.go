package main

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode"
)

const brandDNAPath = "storage/BRAND-DNA.md"

// Groxy AI writes the canonical brand-dna.v1 JSON under this Markdown heading.
// The organization name is the only registration field shared with a website.
func brandOrganization(markdown string) (string, error) {
	const heading = "\n## Profile Data\n\n```json\n"
	invalid := errors.New("Set a business name in the Groxy AI Brand DNA wizard before using prefill.")
	if len(markdown) > 64<<10 {
		return "", invalid
	}
	_, block, found := strings.Cut(strings.ReplaceAll(markdown, "\r\n", "\n"), heading)
	if !found {
		return "", invalid
	}
	profile, rest, found := strings.Cut(block, "\n```")
	if !found || strings.TrimSpace(rest) != "" {
		return "", invalid
	}
	var data struct {
		Schema       string `json:"schema"`
		BusinessName string `json:"business_name"`
	}
	if err := json.Unmarshal([]byte(profile), &data); err != nil || data.Schema != "brand-dna.v1" {
		return "", invalid
	}
	name := strings.TrimSpace(data.BusinessName)
	if name == "" || len(name) > 512 || strings.ContainsFunc(name, unicode.IsControl) {
		return "", invalid
	}
	return name, nil
}
