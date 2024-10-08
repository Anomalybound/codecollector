package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/go-enry/go-enry/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// Config holds the configuration for the code collector
type Config struct {
	IncludeExtensions []string `yaml:"include_extensions"`
	IgnorePatterns    []string `yaml:"ignore_patterns"`
}

// FileData represents the collected data for a single file
type FileData struct {
	RelativePath string `json:"relative_path"`
	Content      string `json:"content"`
}

// CollectedData represents the overall collected data
type CollectedData struct {
	Tree  string     `json:"tree"`
	Files []FileData `json:"files"`
}

// IgnoreRule represents a single ignore rule
type IgnoreRule struct {
	Pattern string
	Source  string // Path of the .gitignore file this rule came from
}

var (
	config            Config
	globalIgnoreRules []IgnoreRule
	rootCmd           = &cobra.Command{Use: "codecollector"}
)

var version string // This will be set by the linker at build time

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ltime | log.Lshortfile)

	rootCmd.PersistentFlags().BoolP("version", "v", false, "Print the version number of Code Collector")
	rootCmd.PersistentFlags().StringP("directory", "d", "", "Path of the folder to traverse")
	rootCmd.PersistentFlags().StringP("output", "o", "collected_code", "Output file name without extension")
	rootCmd.PersistentFlags().String("config", "", "Path to configuration file (YAML)")
	rootCmd.PersistentFlags().String("github", "", "GitHub repository URL to clone and process")
	rootCmd.PersistentFlags().String("branch", "main", "Branch to clone from GitHub repo")
	rootCmd.PersistentFlags().String("output-format", "json", "Output format (json or text)")

	rootCmd.RunE = runCodeCollector
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runCodeCollector(cmd *cobra.Command, args []string) error {
	// Load default config if it exists
	if _, err := os.Stat("config.yaml"); err == nil {
		if err := loadConfig("config.yaml"); err != nil {
			return err
		}
	}

	versionFlag, _ := cmd.Flags().GetBool("version")
	if versionFlag {
		fmt.Printf("Code Collector version %s\n", version)
		return nil
	}

	// Override with user-specified config if provided
	configPath, _ := cmd.Flags().GetString("config")
	if configPath != "" {
		if err := loadConfig(configPath); err != nil {
			return err
		}
	}

	directory, _ := cmd.Flags().GetString("directory")
	githubURL, _ := cmd.Flags().GetString("github")

	var rootDir string
	if githubURL != "" {
		branch, _ := cmd.Flags().GetString("branch")
		clonedDir, err := cloneGitHubRepo(githubURL, branch)
		if err != nil {
			return err
		}
		rootDir = clonedDir
		defer os.RemoveAll(clonedDir)
	} else {
		rootDir = directory
	}

	// Add user-specified ignore patterns
	for _, pattern := range config.IgnorePatterns {
		if pattern != "" {
			globalIgnoreRules = append(globalIgnoreRules, IgnoreRule{Pattern: pattern, Source: "user-config"})
		}
	}

	log.Println("Global ignore rules:")
	for _, rule := range globalIgnoreRules {
		log.Printf("- Pattern: %s, Source: %s", rule.Pattern, rule.Source)
	}

	collectedData, err := collectCode(rootDir)
	if err != nil {
		return err
	}

	outputFormat, _ := cmd.Flags().GetString("output-format")
	outputFile, _ := cmd.Flags().GetString("output")
	return exportOutput(collectedData, outputFile, outputFormat)
}

func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, &config)
}

func cloneGitHubRepo(url, branch string) (string, error) {
	tempDir, err := os.MkdirTemp("", "repo-")
	if err != nil {
		return "", err
	}

	_, err = git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:           url,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Depth:         1,
	})
	if err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	return tempDir, nil
}

func collectCode(rootDir string) (CollectedData, error) {
	log.Println("Starting collectCode function")
	var data CollectedData
	log.Println("Generating tree")
	data.Tree = generateTree(rootDir)
	log.Println("Tree generation complete")

	var wg sync.WaitGroup
	fileChan := make(chan FileData)

	log.Println("Starting filepath.Walk")
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error in filepath.Walk for path %s: %v", path, err)
			return err
		}
		log.Printf("Walking path: %s", path)

		if info.IsDir() {
			if isIgnored(path, rootDir) {
				log.Printf("Directory ignored: %s", path)
				return filepath.SkipDir
			}
			return nil
		}
		if !isIncludedFile(path, rootDir) {
			log.Printf("File not included: %s", path)
			return nil
		}

		log.Printf("Processing file: %s", path)
		wg.Add(1)
		go func() {
			defer wg.Done()
			fileData, err := processFile(path, rootDir)
			if err != nil {
				log.Printf("Error processing file %s: %v", path, err)
				return
			}
			fileChan <- fileData
		}()

		return nil
	})
	log.Println("filepath.Walk completed")

	go func() {
		log.Println("Waiting for all file processing to complete")
		wg.Wait()
		close(fileChan)
		log.Println("All file processing complete, channel closed")
	}()

	log.Println("Collecting processed files")
	for fileData := range fileChan {
		data.Files = append(data.Files, fileData)
	}
	log.Println("All files collected")

	return data, err
}

func isIncludedFile(path, rootDir string) bool {
	if len(config.IncludeExtensions) > 0 {
		ext := filepath.Ext(path)
		for _, includedExt := range config.IncludeExtensions {
			if ext == includedExt {
				return !isIgnored(path, rootDir)
			}
		}
		return false
	}
	return !isIgnored(path, rootDir)
}

func isIgnored(path, rootDir string) bool {
	log.Printf("Checking if ignored: %s", path)
	relPath, err := filepath.Rel(rootDir, path)
	if err != nil {
		log.Printf("Error getting relative path for %s: %v", path, err)
		return false
	}
	relPath = filepath.ToSlash(relPath)

	// Function to check if any part of the path matches the pattern
	matchesAnyPart := func(pattern string) bool {
		parts := strings.Split(relPath, "/")
		for i := range parts {
			subPath := strings.Join(parts[:i+1], "/")
			matched, err := matchPattern(pattern, subPath)
			if err != nil {
				log.Printf("Error matching pattern %s: %v", pattern, err)
				continue
			}
			if matched {
				return true
			}
		}
		return false
	}

	// Check global ignore rules
	for _, rule := range globalIgnoreRules {
		if matchesAnyPart(rule.Pattern) {
			log.Printf("Path %s matched global ignore rule %s", relPath, rule.Pattern)
			return true
		}
	}

	// Check .gitignore files in parent directories
	dir := path
	for {
		if dir == rootDir || dir == "/" || dir == "." {
			break // Stop when we reach the root directory
		}
		gitignorePath := filepath.Join(dir, ".gitignore")
		log.Printf("Checking for .gitignore in: %s", dir)
		if _, err := os.Stat(gitignorePath); err == nil {
			log.Printf("Found .gitignore at: %s", gitignorePath)
			rules, err := parseGitignore(gitignorePath)
			if err != nil {
				log.Printf("Error parsing .gitignore at %s: %v", gitignorePath, err)
			} else {
				relToGitignore, _ := filepath.Rel(dir, path)
				relToGitignore = filepath.ToSlash(relToGitignore)
				for _, rule := range rules {
					if matchesAnyPart(rule.Pattern) {
						log.Printf("Path %s matched .gitignore rule %s from %s", relPath, rule.Pattern, gitignorePath)
						return true
					}
				}
			}
		}
		dir = filepath.Dir(dir)
	}

	log.Printf("Path not ignored: %s", path)
	return false
}

func parseGitignore(path string) ([]IgnoreRule, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rules []IgnoreRule
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			rules = append(rules, IgnoreRule{Pattern: line, Source: path})
		}
	}
	return rules, nil
}

func matchPattern(pattern, path string) (bool, error) {
	// Handle '**' pattern
	if pattern == "**" {
		return true, nil
	}

	// Convert the pattern to a regex
	regexPattern := ""
	for i, char := range pattern {
		switch char {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// '**' should match zero or more directories
				regexPattern += "(?:.*/)?"
				i++ // Skip the next '*'
			} else {
				regexPattern += "[^/]*"
			}
		case '?':
			regexPattern += "[^/]"
		case '.', '+', '(', ')', '|', '[', ']', '{', '}', '^', '$':
			regexPattern += "\\" + string(char)
		case '\\':
			regexPattern += "\\\\"
		default:
			regexPattern += string(char)
		}
	}

	// Anchor the regex
	if !strings.HasPrefix(pattern, "*") {
		regexPattern = "^" + regexPattern
	}
	if !strings.HasSuffix(pattern, "*") {
		regexPattern += "$"
	}

	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return false, fmt.Errorf("invalid pattern %s: %v", pattern, err)
	}

	return regex.MatchString(path), nil
}

func processFile(path, rootDir string) (FileData, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return FileData{}, err
	}

	relPath, err := filepath.Rel(rootDir, path)
	if err != nil {
		return FileData{}, fmt.Errorf("error getting relative path: %v", err)
	}

	return FileData{
		RelativePath: relPath,
		Content:      string(content),
	}, nil
}

func generateTree(root string) string {
	log.Println("Starting generateTree function")
	var output strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error accessing path %s: %v", path, err)
			return err
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			log.Printf("Error getting relative path for %s: %v", path, err)
			return fmt.Errorf("error getting relative path: %v", err)
		}

		log.Printf("Checking path in generateTree: %s", relPath)

		if isIgnored(path, root) {
			log.Printf("Ignored in generateTree: %s", relPath)
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		indent := strings.Repeat("  ", strings.Count(relPath, string(os.PathSeparator)))
		if info.IsDir() {
			log.Printf("Adding directory to tree: %s", relPath)
			output.WriteString(fmt.Sprintf("%s%s/\n", indent, info.Name()))
		} else if isIncludedFile(path, root) {
			log.Printf("Adding file to tree: %s", relPath)
			output.WriteString(fmt.Sprintf("%s%s\n", indent, info.Name()))
		} else {
			log.Printf("Not included in tree: %s", relPath)
		}

		return nil
	})
	if err != nil {
		log.Printf("Error generating tree: %v", err)
		return fmt.Sprintf("Error generating tree: %v", err)
	}
	log.Println("Tree generation complete")
	return output.String()
}

func exportOutput(data CollectedData, outputFile, format string) error {
	switch format {
	case "json":
		return exportJSON(data, outputFile+".json")
	case "text":
		return exportText(data, outputFile+".txt")
	case "markdown":
		return exportMarkdown(data, outputFile+".md")
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func exportJSON(data CollectedData, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func exportText(data CollectedData, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer file.Close()

	file.WriteString("Code Collection Report\n\n")
	file.WriteString("Directory Structure:\n\n")
	file.WriteString(data.Tree)
	file.WriteString("\n")

	for _, fileData := range data.Files {
		file.WriteString(fmt.Sprintf("File: %s\n\n", fileData.RelativePath))
		file.WriteString("Content:\n\n")
		file.WriteString(fileData.Content)
		file.WriteString("\n\n")
		file.WriteString(strings.Repeat("-", 80) + "\n\n")
	}

	return nil
}

func exportMarkdown(data CollectedData, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer file.Close()

	// Write title
	file.WriteString("# Code Collection Report\n\n")

	// Write directory structure
	file.WriteString("## Directory Structure\n\n")
	file.WriteString("```plaintext\n")
	file.WriteString(data.Tree)
	file.WriteString("```\n\n")

	// Write file contents
	file.WriteString("## File Contents\n\n")
	for _, fileData := range data.Files {
		file.WriteString(fmt.Sprintf("### %s\n\n", fileData.RelativePath))

		// Detect language using go-enry
		language, _ := enry.GetLanguageByExtension(fileData.RelativePath)
		if language == "" {
			// If extension doesn't give a result, try content-based detection
			language = enry.GetLanguage(fileData.RelativePath, []byte(fileData.Content))
		}

		// If still no language detected, default to "plaintext"
		if language == "" {
			language = "plaintext"
		}

		file.WriteString(fmt.Sprintf("```%s\n", language))
		file.WriteString(fileData.Content)
		file.WriteString("\n```\n\n")
	}

	return nil
}
