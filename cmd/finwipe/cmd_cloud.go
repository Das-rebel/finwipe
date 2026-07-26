package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var cloudCmd = &cobra.Command{
	Use:   "cloud-status",
	Short: "Check FinWipe cloud connectivity and configuration",
	Long: `Check if FinWipe cloud is properly configured and accessible.

Shows:
  - Whether cloud endpoint is reachable
  - Whether inbox is configured
  - API key status
  - KV storage stats (if accessible)`,
	RunE: runCloudStatus,
}

func init() {
	rootCmd.AddCommand(cloudCmd)
}

func runCloudStatus(cmd *cobra.Command, args []string) error {
	// Load inbox config
	inboxPath := filepath.Join(os.Getenv("HOME"), ".finwipe", "inbox")
	inboxData, err := os.ReadFile(inboxPath)
	hasInbox := err == nil

	var inboxAddr, userHash, apiKey, cloudEndpoint string
	if hasInbox {
		lines := strings.Split(strings.TrimSpace(string(inboxData)), "\n")
		if len(lines) >= 1 {
			inboxAddr = lines[0]
		}
		if len(lines) >= 2 {
			userHash = lines[1]
		}
		if len(lines) >= 3 {
			apiKey = strings.TrimSpace(lines[2])
		}
		if len(lines) >= 4 {
			cloudEndpoint = lines[3]
		}
	}
	if cloudEndpoint == "" {
		cloudEndpoint = "https://fw.finwipe.in"
	}

	fmt.Println()
	fmt.Println("  ☁️  FinWipe Cloud Status")
	fmt.Println("  ─────────────────────────────────────────────────────────────")

	if !hasInbox {
		fmt.Println("  ⚠️  Inbox not configured")
		fmt.Println()
		fmt.Println("  Run: finwipe setup-forward")
		fmt.Println()
		return nil
	}

	fmt.Printf("  📧 Inbox:    %s\n", inboxAddr)
	fmt.Printf("  🔑 User ID:  %s\n", userHash)
	fmt.Printf("  🔗 Cloud:    %s\n", cloudEndpoint)
	if apiKey != "" {
		fmt.Printf("  🔐 API Key:  %s...\n", apiKey[:8])
	} else {
		fmt.Printf("  🔐 API Key:  not set\n")
	}

	// Test cloud connectivity
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────")
	fmt.Println("  🌐 Connectivity Tests")
	fmt.Println("  ─────────────────────────────────────────────────────────────")

	client := &http.Client{Timeout: 10 * time.Second}

	// Test health endpoint
	healthURL := strings.TrimSuffix(cloudEndpoint, "/") + "/api/health"
	resp, err := client.Get(healthURL)
	if err != nil {
		fmt.Printf("  ❌ Health:    UNREACHABLE (%v)\n", err)
		fmt.Println()
		fmt.Println("  Check:")
		fmt.Println("    1. Worker deployed? (run deploy.sh)")
		fmt.Println("    2. Cloudflare dashboard: https://dash.cloudflare.com")
		fmt.Println("    3. Worker URL correct? Edit ~/.finwipe/inbox line 4")
	} else {
		body, _ := io.ReadAll(resp.Body)
		var health struct {
			Status    string `json:"status"`
			Timestamp string `json:"timestamp"`
		}
		json.Unmarshal(body, &health)
		if health.Status == "ok" {
			fmt.Printf("  ✅ Health:    OK (since %s)\n", health.Timestamp)
		} else {
			fmt.Printf("  ⚠️  Health:    %s\n", string(body))
		}
		resp.Body.Close()
	}

	// Test discoveries endpoint
	if userHash != "" {
		discURL := strings.TrimSuffix(cloudEndpoint, "/") + "/api/discoveries?user_id=" + userHash
		if apiKey != "" {
			discURL += "&api_key=" + apiKey
		}
		resp, err := client.Get(discURL)
		if err != nil {
			fmt.Printf("  ❌ Discoveries: UNREACHABLE (%v)\n", err)
		} else {
			body, _ := io.ReadAll(resp.Body)
			var result struct {
				Count int `json:"count"`
			}
			json.Unmarshal(body, &result)
			if resp.StatusCode == 200 {
				fmt.Printf("  ✅ Discoveries: OK (%d FIs found)\n", result.Count)
			} else {
				fmt.Printf("  ⚠️  Discoveries: HTTP %d (%s)\n", resp.StatusCode, string(body))
			}
			resp.Body.Close()
		}
	}

	// Summary
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────")

	// Check Gmail filter
	fmt.Println()
	fmt.Println("  📋 SETUP CHECKLIST:")
	fmt.Println()
	fmt.Println("  ☐ Gmail filter not set up")
	fmt.Println("    Settings → Filters → Create filter:")
	fmt.Printf("      Has: bank OR loan OR EMI OR credit OR insurance\n")
	fmt.Printf("      Forward to: %s\n", inboxAddr)
	fmt.Println()
	fmt.Println("  ☐ Cloudflare Worker not deployed")
	fmt.Println("    cd apps/finwipe-cloud && ./deploy.sh")
	fmt.Println()
	fmt.Println("  ☐ Mailgun not configured")
	fmt.Println("    https://app.mailgun.com/app/inbox")
	fmt.Println("    Set route: catch_all() → forward to /api/forward")
	fmt.Println()

	return nil
}
