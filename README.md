# Code Collector

Code Collector is a powerful and flexible tool designed to traverse directories, collect code files, and generate comprehensive reports. It's particularly useful for developers, code reviewers, and anyone needing to analyze or document code structures across projects.

## Features

- **Directory Traversal**: Recursively explore directory structures.
- **Selective File Collection**: Customize which files to include based on extensions and ignore patterns.
- **Git Integration**: Clone and analyze GitHub repositories directly.
- **Gitignore Support**: Respects `.gitignore` rules at multiple directory levels.
- **Flexible Output Formats**: Generate reports in JSON, plain text, or Markdown.
- **Tree View Generation**: Create a visual representation of the directory structure.
- **Language Detection**: Automatically detect and highlight code based on file type.
- **Concurrent Processing**: Utilize goroutines for efficient file processing.
- **Versioning**: Semantic versioning for better release management and compatibility.

## Installation

To install Code Collector, ensure you have Go installed on your system, then run:

```bash
go get -u github.com/Anomalybound/codecollector
```

To install a specific version:

```bash
go get -u github.com/Anomalybound/codecollector@v1.0.0
```

## Usage

### Basic Usage

```bash
codecollector -d /path/to/your/directory -o output_file
```

### Command-Line Options

- `-d, --directory`: Path of the folder to traverse (required if not using `--github`)
- `-o, --output`: Output file name without extension (default: "collected_code")
- `--config`: Path to configuration file (YAML)
- `--github`: GitHub repository URL to clone and process
- `--branch`: Branch to clone from GitHub repo (default: "main")
- `--output-format`: Output format (json, text, or markdown)
- `-v, --version`: Display the current version of Code Collector

### Using with GitHub Repositories

To analyze a GitHub repository:

```bash
codecollector --github https://github.com/user/repo --branch main -o github_report
```

### Configuration File

You can use a YAML configuration file to set include extensions and ignore patterns:

```yaml
include_extensions:
  - .go
  - .js
  - .py
ignore_patterns:
  - "vendor/"
  - "node_modules/"
```

Use the configuration file with:

```bash
codecollector -d /path/to/project --config config.yaml -o configured_output
```

## Output Formats

### JSON

JSON output includes a tree representation of the directory structure and the content of each collected file.

### Text

Plain text output provides a readable format with the directory structure and file contents.

### Markdown

Markdown output creates a structured document with syntax highlighting for code snippets, making it ideal for documentation or sharing on platforms like GitHub.

## Development

### Prerequisites

- Go 1.16 or higher
- Dependencies (automatically installed with `go get`):
  - github.com/go-enry/go-enry/v2
  - github.com/go-git/go-git/v5
  - github.com/spf13/cobra
  - gopkg.in/yaml.v2

### Building from Source

1. Clone the repository:

   ```bash
   git clone git@github.com:Anomalybound/codecollector.git
   ```

2. Navigate to the project directory:

   ```bash
   cd codecollector
   ```

3. Build the project:
   ```bash
   go build
   ```

## Versioning

Code Collector follows [Semantic Versioning](https://semver.org/). Version numbers are in the format MAJOR.MINOR.PATCH.

To see the current version:

```bash
codecollector --version
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

When contributing, please follow the existing code style and add tests for any new features. Make sure to update the CHANGELOG.md file with your changes under the "Unreleased" section.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [go-enry](https://github.com/go-enry/go-enry) for language detection
- [go-git](https://github.com/go-git/go-git) for Git operations
- [cobra](https://github.com/spf13/cobra) for CLI interface

## Support

If you encounter any problems or have any questions, please open an issue on the GitHub repository at https://github.com/Anomalybound/codecollector/issues.
