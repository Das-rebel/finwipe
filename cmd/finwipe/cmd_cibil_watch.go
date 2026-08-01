package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var cibilWatchCmd = &cobra.Command{
	Use:   "cibl-watch",
	Short: "Scrape CIBIL Watch — see who accessed your credit report",
	Long: `Check CIBIL Watch to see which lenders have accessed your credit report.

CIBIL Watch shows all member accesses to your CIBIL report in the last 24 months.

This command requires:
1. Your CIBIL login credentials (Member ID + password)
2. Session cookies from a logged-in CIBIL session

For manual access:
  https://consumer.cibil.com → CIBIL Watch

Or use browser-based import:
  finwipe cibil-watch --browser
`,
	RunE: runCibilWatch,
}

var (
	cibilWatchSession string
	cibilWatchBrowser bool
)

func init() {
	rootCmd.AddCommand(cibilWatchCmd)
	cibilWatchCmd.Flags().BoolVar(&cibilWatchBrowser, "browser", false,
		"Open browser to login to CIBIL and import session")
	cibilWatchCmd.Flags().StringVar(&cibilWatchSession, "session", "",
		"Path to session JSON file (exported from browser)")
}

func runCibilWatch(cmd *cobra.Command, args []string) error {
	fmt.Print(`
🏦 CIBIL Watch — Access History

CIBIL Watch shows every lender who has queried your credit report.
This helps you audit who knows your financial profile.

─────────────────────────────────────────
MANUAL METHOD (Always works):
─────────────────────────────────────────
1. Go to: https://consumer.cibil.com
2. Login with your CIBIL Member ID and password
3. Click: "CIBIL Watch" → "View Access History"
4. Export or screenshot the list of member accesses

─────────────────────────────────────────
AUTOMATED METHOD (Requires setup):
─────────────────────────────────────────
1. Export CIBIL cookies from your browser
2. Save as: ~/.finwipe/cibil-session.json
3. Run: finwipe cibil-watch

To export CIBIL cookies:
  • Chrome: Install "EditThisCookie" extension
  • Go to consumer.cibil.com and login
  • Export cookies as JSON
  • Save to ~/.finwipe/cibil-session.json
`,
	)

	// Check for session file
	sessionPath := cibilWatchSession
	if sessionPath == "" {
		sessionPath = filepathJoin(os.Getenv("HOME"), ".finwipe", "cibil-session.json")
	}

	sessionFile, err := os.Open(sessionPath)
	if err != nil {
		fmt.Printf("\n📁 No session file at %s\n", sessionPath)
		fmt.Println("   Use --session /path/to/session.json to specify")
		return nil
	}
	defer sessionFile.Close()

	fmt.Println("\n🔍 Attempting to fetch CIBIL Watch data...")

	session, err := parseCibilSession(sessionFile)
	if err != nil {
		return fmt.Errorf("parse session: %w", err)
	}

	accesses, err := fetchCibilWatch(session)
	if err != nil {
		fmt.Printf("⚠️  Could not fetch: %v\n", err)
		fmt.Println("   Use manual method above")
		return nil
	}

	fmt.Printf("\n📊 CIBIL Watch — %d Member Accesses Found\n", len(accesses))
	fmt.Println("────────────────────────────────────────────────────────────")
	fmt.Println(" Date        | Member Name                     | Purpose")
	fmt.Println("────────────────────────────────────────────────────────────")
	for _, a := range accesses {
		fmt.Printf(" %s | %-30s | %s\n",
			a.Date.Format("02 Jan 2006"), a.Member, a.Purpose)
	}

	// Save to file
	reportPath := filepathJoin(os.Getenv("HOME"), ".finwipe", "cibil-watch.json")
	data, _ := json.MarshalIndent(accesses, "", "  ")
	os.WriteFile(reportPath, data, 0600)
	fmt.Printf("\n💾 Saved to: %s\n", reportPath)
	return nil
}

type CibilAccess struct {
	Date    time.Time
	Member  string
	Purpose string
}

func parseCibilSession(r io.Reader) (map[string]string, error) {
	var sess map[string]string
	if err := json.NewDecoder(r).Decode(&sess); err != nil {
		// Try as cookie array format
		var cookies []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r).Decode(&cookies); err != nil {
			return nil, err
		}
		sess = make(map[string]string)
		for _, c := range cookies {
			sess[c.Name] = c.Value
		}
	}
	return sess, nil
}

func fetchCibilWatch(session map[string]string) ([]CibilAccess, error) {
	// Try to fetch CIBIL Watch page
	req, _ := http.NewRequest("GET", "https://consumer.cibil.com/cibil-watch", nil)
	for k, v := range session {
		req.Header.Set("Cookie", fmt.Sprintf("%s=%s", k, v))
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 FinWipe/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("CIBIL returned status %d (session may be expired)", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Parse access history from HTML
	var accesses []CibilAccess

	// Try JSON first (API response)
	accesses = parseCibilWatchJSON(html)
	if len(accesses) > 0 {
		return accesses, nil
	}

	// Fallback: parse HTML table
	accesses = parseCibilWatchHTML(html)
	return accesses, nil
}

func parseCibilWatchJSON(html string) []CibilAccess {
	// Look for JSON data in the page
	re := regexp.MustCompile(`"accessHistory"\s*:\s*\[(.*?)\]`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil
	}

	var accesses []CibilAccess
	re2 := regexp.MustCompile(`\{"memberName"\s*:\s*"([^"]+)"[^}]*"inquiryDate"\s*:\s*"([^"]+)"[^}]*"purpose"\s*:\s*"([^"]+)"`)
	for _, m := range re2.FindAllStringSubmatch(matches[1], -1) {
		t, _ := time.Parse("2006-01-02", m[2])
		accesses = append(accesses, CibilAccess{
			Date:    t,
			Member:  m[1],
			Purpose: m[3],
		})
	}
	return accesses
}

func parseCibilWatchHTML(html string) []CibilAccess {
	var accesses []CibilAccess
	// Simple HTML table parsing for access history
	rows := regexp.MustCompile(`<tr[^>]*>(.*?)</tr>`).FindAllStringSubmatch(html, -1)
	for _, row := range rows {
		cells := regexp.MustCompile(`<td[^>]*>(.*?)</td>`).FindAllStringSubmatch(row[1], -1)
		if len(cells) >= 3 {
			member := stripTags(cells[0][1])
			dateStr := stripTags(cells[1][1])
			purpose := stripTags(cells[2][1])
			if member != "" && dateStr != "" {
				t, _ := time.Parse("02-Jan-2006", strings.TrimSpace(dateStr))
				accesses = append(accesses, CibilAccess{
					Date:    t,
					Member:  strings.TrimSpace(member),
					Purpose: strings.TrimSpace(purpose),
				})
			}
		}
	}
	return accesses
}

func stripTags(s string) string {
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return strings.TrimSpace(s)
}

func filepathJoin(elem ...string) string {
	if len(elem) == 0 {
		return ""
	}
	result := elem[0]
	for _, e := range elem[1:] {
		if !strings.HasSuffix(result, "/") && !strings.HasSuffix(result, "\\") {
			result += "/"
		}
		result += e
	}
	return result
}
