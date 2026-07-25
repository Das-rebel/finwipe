package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var parseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parse a CIBIL credit report PDF to extract NBFC names",
	RunE:  runParse,
}

func runParse(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: finwipe parse <credit_report.pdf>")
	}

	pdfPath := args[0]
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Extract text — basic approach (no PDF library needed for text extraction)
	text := extractText(data)

	// Find loan accounts section
	// Look for section headers and account names
	var nbfcs []string
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) < 3 {
			continue
		}

		// Known NBFC patterns
		nbfcNames := []string{
			"Bajaj", "Tata Capital", "Aditya Birla", "L&T Finance", "HDB Financial",
			"Cholamandalam", "Mahindra Finance", "Shriram", "Muthoot", "Navi Finserv",
			"Kissht", "StashFin", "LazeeAp", "Krazen", "Rubicon",
			"IndusInd Bank", "HDFC Bank", "ICICI Bank", "Axis Bank",
			"Kotak", "Yes Bank", "IDBI",
		}

		for _, name := range nbfcNames {
			if strings.Contains(trimmed, name) && len(trimmed) < 100 {
				nbfcs = append(nbfcs, trimmed)
			}
		}

		// Also look for EMI patterns: "EMI" + institution name nearby
		if strings.Contains(trimmed, "EMI") && i > 0 {
			prev := strings.TrimSpace(lines[i-1])
			for _, name := range nbfcNames {
				if strings.Contains(prev, name) {
					nbfcs = append(nbfcs, prev)
				}
			}
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, n := range nbfcs {
		lower := strings.ToLower(n)
		if !seen[lower] {
			seen[lower] = true
			unique = append(unique, n)
		}
	}

	fmt.Printf("\n📋 NBFCs Detected in Credit Report (%d)\n\n", len(unique))
	for _, n := range unique {
		fmt.Printf("  • %s\n", n)
	}
	fmt.Println()

	if len(unique) == 0 {
		fmt.Println("⚠️  No NBFCs detected. Try:")
		fmt.Println("  • Check the PDF is a CIBIL/credit report")
		fmt.Println("  • Use --text flag if PDF is text-based")
	}

	return nil
}

func extractText(data []byte) string {
	// Basic PDF text extraction — finds text between BT/ET markers
	text := ""
	re := regexp.MustCompile(`BT\s*(.*?)\s*ET`)
	matches := re.FindAllSubmatch(data, -1)
	for _, m := range matches {
		// Extract strings from Tj and TJ operators
		str := string(m[1])
		// Remove formatting
		str = regexp.MustCompile(`\[|\]|\(|\)|\{|\}`).ReplaceAllString(str, " ")
		str = regexp.MustCompile(`\\[\w]`).ReplaceAllString(str, "")
		text += str + "\n"
	}
	return text
}
