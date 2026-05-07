package skill

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	searchTimeout    = 10 * time.Second
	searchMaxResults = 10
)

var (
	ddgResultRe  = regexp.MustCompile(`<a rel="nofollow" class="result__a" href="([^"]*)"[^>]*>(.*?)</a>`)
	ddgSnippetRe = regexp.MustCompile(`<a class="result__snippet"[^>]*>(.*?)</a>`)
	htmlTagRe    = regexp.MustCompile(`<[^>]*>`)
)

type webSearchSkill struct {
	// searxngEndpoint, if non-empty, routes search through a local
	// (or remote) SearXNG meta-search instance. The DDG HTML scrape
	// stays as a fallback for when SearXNG is unreachable.
	// Set via $SEARXNG_ENDPOINT (e.g. http://localhost:8080).
	searxngEndpoint string
}

func NewWebSearchSkill() Skill {
	return &webSearchSkill{
		searxngEndpoint: strings.TrimRight(os.Getenv("SEARXNG_ENDPOINT"), "/"),
	}
}

func (s *webSearchSkill) Name() string { return "web_search" }
func (s *webSearchSkill) Description() string {
	if s.searxngEndpoint != "" {
		return fmt.Sprintf(
			"Search the web via local SearXNG meta-search at %s "+
				"(privacy-respecting, aggregates DuckDuckGo / Google / "+
				"Bing / Yahoo / etc.). Returns titles, URLs, snippets. "+
				"Falls back to DuckDuckGo HTML scrape if SearXNG is "+
				"unreachable.", s.searxngEndpoint)
	}
	return "Search the web using DuckDuckGo and return results with " +
		"titles, URLs, and snippets. No API key needed. Set the " +
		"SEARXNG_ENDPOINT env var (e.g. http://localhost:8080) to " +
		"route through a local SearXNG meta-search for better " +
		"reliability + multi-engine aggregation."
}
func (s *webSearchSkill) ToolDef() json.RawMessage {
	return MakeToolDef("web_search", s.Description(),
		map[string]map[string]string{
			"query": {"type": "string", "description": "The search query"},
		}, []string{"query"})
}

func (s *webSearchSkill) Execute(params map[string]string) (string, error) {
	query := params["query"]
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	// Backend dispatch: SearXNG first when configured, then DDG as
	// fallback. Both can fail — DDG in particular has been returning
	// HTTP 202 + an empty homepage shell since mid-2026 (anti-bot
	// guard against the html.duckduckgo.com endpoint). When ALL
	// backends fail, the error message tells the model what to do
	// next instead of just dying — the researcher soul's fallback
	// chain points to web_scrape (Scrapling, browser TLS fingerprint,
	// can hit duckduckgo.com directly and parse SERP) as plan B.
	var (
		results       []searchResult
		backend       string
		searxngErr    error
		ddgErr        error
		searxngTried  bool
	)
	if s.searxngEndpoint != "" {
		searxngTried = true
		r, err := searchSearXNG(s.searxngEndpoint, query)
		if err == nil {
			results, backend = r, "searxng"
		} else {
			searxngErr = err
			ddg, e := searchDuckDuckGo(query)
			if e == nil {
				results = ddg
				backend = fmt.Sprintf("duckduckgo (searxng unreachable: %v)", err)
			} else {
				ddgErr = e
			}
		}
	} else {
		r, err := searchDuckDuckGo(query)
		if err != nil {
			ddgErr = err
		} else {
			results, backend = r, "duckduckgo"
		}
	}

	// Both backends down — surface an actionable message. The
	// researcher soul's fallback chain reads this and pivots to
	// web_scrape rather than spinning on web_search.
	if results == nil && (searxngErr != nil || ddgErr != nil) {
		var msg strings.Builder
		msg.WriteString("web_search backends all unreachable:\n")
		if searxngTried {
			fmt.Fprintf(&msg, "  - searxng %s: %v\n", s.searxngEndpoint, searxngErr)
			msg.WriteString("    → if you run a local SearXNG, restart it (e.g. `docker restart searxng`).\n")
		}
		if ddgErr != nil {
			fmt.Fprintf(&msg, "  - duckduckgo html scrape: %v\n", ddgErr)
			msg.WriteString("    → DDG's html.duckduckgo.com endpoint blocks non-browser TLS in 2026.\n")
		}
		msg.WriteString("\nFallback: call `web_scrape` with url=\"https://duckduckgo.com/?q=<query>\" — Scrapling has browser TLS fingerprinting and can bypass DDG's bot guard. Or hit a specific source directly via `web_fetch` if you already have a URL in mind.")
		return "", fmt.Errorf("%s", msg.String())
	}

	if len(results) == 0 {
		return "No results found (via " + backend + ").", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for %q (via %s):\n\n", query, backend))
	for i, r := range results {
		if i >= searchMaxResults {
			break
		}
		sb.WriteString(fmt.Sprintf("%d. %s\n   %s\n   %s\n\n", i+1, r.title, r.url, r.snippet))
	}

	return "---BEGIN UNTRUSTED WEB CONTENT---\n" +
		strings.TrimSpace(sb.String()) +
		"\n---END UNTRUSTED WEB CONTENT---", nil
}

// searchSearXNG queries a local or remote SearXNG meta-search instance
// at /search?q=...&format=json. Response shape:
//
//	{
//	  "query": "...",
//	  "results": [
//	    {"url": "...", "title": "...", "content": "...", ...},
//	    ...
//	  ]
//	}
//
// SearXNG already dedupes across engines and ranks; we just take the
// top N as-is.
func searchSearXNG(endpoint, query string) ([]searchResult, error) {
	u := endpoint + "/search?format=json&q=" + url.QueryEscape(query)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: searchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searxng request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("searxng HTTP %d", resp.StatusCode)
	}

	var raw struct {
		Results []struct {
			URL     string `json:"url"`
			Title   string `json:"title"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("searxng decode: %w", err)
	}

	out := make([]searchResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		out = append(out, searchResult{
			title:   strings.TrimSpace(r.Title),
			url:     r.URL,
			snippet: strings.TrimSpace(r.Content),
		})
	}
	return out, nil
}

type searchResult struct {
	title, url, snippet string
}

func searchDuckDuckGo(query string) ([]searchResult, error) {
	req, err := http.NewRequest("GET", "https://html.duckduckgo.com/html/?q="+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Referer", "https://duckduckgo.com/")

	client := &http.Client{Timeout: searchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DuckDuckGo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DuckDuckGo returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(body)

	linkMatches := ddgResultRe.FindAllStringSubmatch(html, -1)
	snippetMatches := ddgSnippetRe.FindAllStringSubmatch(html, -1)

	var results []searchResult
	for i, m := range linkMatches {
		if len(m) < 3 {
			continue
		}
		rawURL := m[1]
		title := stripHTML(m[2])

		if parsed, err := url.Parse(rawURL); err == nil {
			if real := parsed.Query().Get("uddg"); real != "" {
				rawURL = real
			}
		}

		snippet := ""
		if i < len(snippetMatches) && len(snippetMatches[i]) >= 2 {
			snippet = stripHTML(snippetMatches[i][1])
		}

		results = append(results, searchResult{title: title, url: rawURL, snippet: snippet})
	}
	return results, nil
}

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<b>", "")
	s = strings.ReplaceAll(s, "</b>", "")
	s = htmlTagRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
