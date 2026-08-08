package scraper

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// htmlToText renders an HTML fragment (such as a JobPosting/ATS description
// field) down to normalized plain text: tags are dropped and whitespace is
// collapsed. Input that is already plain text passes through unchanged. It is
// deterministic and AI-free, used by the Faz 10 structured-data adapters.
func htmlToText(fragment string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	context := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(fragment), context)
	if err != nil {
		return fragment
	}
	parts := make([]string, 0)
	for _, node := range nodes {
		if text := normalizedText(node); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}
