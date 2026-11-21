package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"strings"

	"groxy-desk/internal/security"
)

type paneCommand struct {
	Type  string `json:"type"`
	URL   string `json:"url,omitempty"`
	Width int    `json:"width,omitempty"`
}

func trustedWorkbenchSource(raw, base string) bool {
	if raw != base+"/" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	_, err = security.ExactOrigin(u)
	return err == nil
}

func trustedWorkbenchResource(raw, base string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	origin, err := security.ExactOrigin(u)
	return err == nil && origin == base
}

func decodePaneCommand(raw string) (paneCommand, bool) {
	if len(raw) > 4096 {
		return paneCommand{}, false
	}
	dec := json.NewDecoder(bytes.NewBufferString(raw))
	dec.DisallowUnknownFields()
	var cmd paneCommand
	if dec.Decode(&cmd) != nil || dec.Decode(new(any)) != io.EOF {
		return paneCommand{}, false
	}
	switch cmd.Type {
	case "groxy-desk-ready", "groxy-browser-close", "groxy-browser-capture", "groxy-browser-brand-preview", "groxy-browser-brand-prefill":
		return cmd, cmd.URL == "" && cmd.Width == 0
	case "groxy-browser-open":
		return cmd, cmd.URL != "" && cmd.Width == 0
	case "groxy-browser-resize":
		return cmd, cmd.URL == "" && cmd.Width >= 30 && cmd.Width <= 65
	}
	return paneCommand{}, false
}

type browserOrigins map[string]bool

func parseBrowserOrigins(raw ...string) browserOrigins {
	origins := make(browserOrigins)
	for _, group := range raw {
		for _, entry := range strings.Split(group, ",") {
			if origin, err := security.ParseServiceOrigin(strings.TrimSpace(entry), false); err == nil {
				origins[origin.Identity()] = true
			}
		}
	}
	return origins
}

func (origins browserOrigins) match(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return ""
	}
	origin, err := security.ExactOrigin(u)
	if err != nil || !origins[origin] {
		return ""
	}
	return origin
}
