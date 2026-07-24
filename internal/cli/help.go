package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tillandsia/tillandsia"
)

func init() {
	rootCmd.AddCommand(helpCmd)
}

var helpCmd = &cobra.Command{
	Use:   "help [topic]",
	Short: "Show documentation for a topic",
	Long: `Display embedded documentation. Topics include:
  architecture, config, concepts, deploy, dns, env, faq, init, server

Use 'help topics' to list all topics or 'help examples' for quickstarts.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		topic := args[0]

		if topic == "topics" || topic == "examples" {
			return listDocs(topic)
		}

		path := fmt.Sprintf("docs/%s.md", topic)
		content, err := tillandsia.DocsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("no documentation found for %q", topic)
		}

		cmd.Print(string(content))
		return nil
	},
}

func listDocs(kind string) error {
	var entries []string
	base := "docs"
	if kind == "examples" {
		base = "docs/examples"
	}

	if err := fs.WalkDir(tillandsia.DocsFS, base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			match, _ := filepath.Match("*.md", filepath.Base(path))
			if match {
				rel, _ := filepath.Rel("docs", path)
				entries = append(entries, rel)
			}
		}
		return nil
	}); err != nil {
		return err
	}

	sort.Strings(entries)
	if kind == "examples" {
		if jsonOutput {
			fmt.Println(formatJSONList(entries, "tillandsia help examples/"))
			return nil
		}
		fmt.Println("Available example quickstarts:")
		for _, e := range entries {
			fmt.Printf("  tillandsia help %s\n", strings.TrimSuffix(e, ".md"))
		}
	} else {
		entries = filter(entries, func(s string) bool {
			return !strings.HasPrefix(s, "examples")
		})
		if jsonOutput {
			fmt.Println(formatJSONList(entries, "tillandsia help "))
			return nil
		}
		fmt.Println("Available topics:")
		for _, e := range entries {
			name := strings.TrimSuffix(e, ".md")
			fmt.Printf("  tillandsia help %s\n", name)
		}
	}
	return nil
}

func filter(ss []string, fn func(string) bool) []string {
	var out []string
	for _, s := range ss {
		if fn(s) {
			out = append(out, s)
		}
	}
	return out
}

func formatJSONList(entries []string, prefix string) string {
	type entry struct {
		Name string `json:"name"`
		Cmd  string `json:"command"`
	}
	var items []entry
	for _, e := range entries {
		name := strings.TrimSuffix(e, ".md")
		items = append(items, entry{Name: name, Cmd: prefix + name})
	}
	b, _ := json.MarshalIndent(items, "", "  ")
	return string(b)
}
