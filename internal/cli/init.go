package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/tillandsia/tillandsia/internal/config"
	"github.com/tillandsia/tillandsia/internal/types"
)

type detectedLanguage string

const (
	langNode    detectedLanguage = "node"
	langPython  detectedLanguage = "python"
	langGo      detectedLanguage = "go"
	langRuby    detectedLanguage = "ruby"
	langStatic  detectedLanguage = "static"
	langUnknown detectedLanguage = "unknown"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new Tillandsia project",
	Long: `Detect your project's language and generate Dockerfile, Procfile,
and tillandsia.yaml. If the directory is empty, a sample app is created.

Supported languages: node, python, go, ruby, static`,
	RunE: func(cmd *cobra.Command, args []string) error {
		langFlag, _ := cmd.Flags().GetString("language")
		yesFlag, _ := cmd.Flags().GetBool("yes")

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		empty, err := isDirEmpty(dir)
		if err != nil {
			return err
		}

		lang := detectLanguage(dir)
		if langFlag != "" {
			lang = detectedLanguage(langFlag)
		}

		if !yesFlag {
			fmt.Printf("Detected language: %s\n", lang)
			if lang == langUnknown {
				fmt.Println("Could not detect language. Using generic template.")
			}
		}

		if empty {
			if err := scaffoldSampleApp(dir, lang); err != nil {
				return err
			}
		}

		appName := filepath.Base(dir)

		cfg := &types.Config{
			Name: appName,
			Port: 8080,
			Env:  map[string]string{},
			Build: types.BuildConfig{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
		}
		if err := config.WriteConfig(dir, cfg); err != nil {
			return err
		}

		services := []*types.Service{
			{Type: types.ServiceTypeWeb, Command: detectStartCommand(lang, appName)},
		}
		if err := config.WriteProcfile(dir, services); err != nil {
			return err
		}

		if err := scaffoldDockerfile(dir, lang); err != nil {
			return err
		}

		fmt.Printf("Initialized Tillandsia project in %s\n", dir)
		return nil
	},
}

func init() {
	initCmd.Flags().String("language", "", "Force a specific language (node, python, go, ruby, static)")
	initCmd.Flags().Bool("yes", false, "Non-interactive mode, accept defaults")
}

func detectLanguage(dir string) detectedLanguage {
	entries := listDir(dir)
	for _, e := range entries {
		switch e {
		case "package.json":
			return langNode
		case "requirements.txt", "pyproject.toml", "Pipfile":
			return langPython
		case "go.mod":
			return langGo
		case "Gemfile":
			return langRuby
		}
	}
	for _, e := range entries {
		if e == "index.html" {
			return langStatic
		}
	}
	return langUnknown
}

func detectStartCommand(lang detectedLanguage, appName string) string {
	switch lang {
	case langNode:
		if hasFile("server.js") {
			return "node server.js"
		}
		if hasFile("index.js") {
			return "node index.js"
		}
		if hasFile("app.js") {
			return "node app.js"
		}
		return "node server.js"
	case langPython:
		if hasFile("app.py") {
			return "python app.py"
		}
		if hasFile("main.py") {
			return "python main.py"
		}
		return "python app.py"
	case langGo:
		return "./" + appName
	case langRuby:
		if hasFile("config.ru") {
			return "bundle exec rackup config.ru -p $PORT"
		}
		return "ruby app.rb"
	case langStatic:
		return "echo 'Static site served by nginx' && sleep infinity"
	case langUnknown:
		return "node server.js"
	default:
		return "node server.js"
	}
}

func hasFile(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

func listDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func isDirEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func scaffoldSampleApp(dir string, lang detectedLanguage) error {
	switch lang {
	case langNode:
		return writeFiles(dir, map[string]string{
			"server.js": sampleNodeJS,
		})
	case langPython:
		return writeFiles(dir, map[string]string{
			"app.py": samplePython,
		})
	case langGo:
		return writeFiles(dir, map[string]string{
			"main.go": sampleGo,
		})
	case langRuby:
		return writeFiles(dir, map[string]string{
			"app.rb": sampleRuby,
		})
	case langStatic:
		return writeFiles(dir, map[string]string{
			"index.html": sampleHTML,
		})
	default:
		return writeFiles(dir, map[string]string{
			"server.js": sampleNodeJS,
		})
	}
}

func scaffoldDockerfile(dir string, lang detectedLanguage) error {
	var dockerfile string
	switch lang {
	case langNode:
		dockerfile = dockerfileNode
	case langPython:
		dockerfile = dockerfilePython
	case langGo:
		dockerfile = dockerfileGo
	case langRuby:
		dockerfile = dockerfileRuby
	case langStatic:
		dockerfile = dockerfileStatic
	default:
		dockerfile = dockerfileNode
	}
	return os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0644)
}

func writeFiles(dir string, files map[string]string) error {
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

const sampleNodeJS = `const http = require('http');
const port = process.env.PORT || 8080;

const server = http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': 'text/html' });
  res.end('<h1>Hello from Tillandsia!</h1>');
});

server.listen(port, () => {
  console.log('Server running on port', port);
});
`

const samplePython = `import os
from http.server import HTTPServer, SimpleHTTPRequestHandler

port = int(os.environ.get('PORT', 8080))
server = HTTPServer(('0.0.0.0', port), SimpleHTTPRequestHandler)
print(f'Server running on port {port}')
server.serve_forever()
`

const sampleGo = `package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<h1>Hello from Tillandsia!</h1>")
	})
	fmt.Println("Server running on port", port)
	http.ListenAndServe(":"+port, nil)
}
`

const sampleRuby = `require 'socket'
require 'webrick'

port = ENV.fetch('PORT', 8080).to_i
server = WEBrick::HTTPServer.new(Port: port)
server.mount_proc '/' do |req, res|
  res.body = '<h1>Hello from Tillandsia!</h1>'
end
trap('INT') { server.shutdown }
puts "Server running on port #{port}"
server.start
`

const sampleHTML = `<!DOCTYPE html>
<html>
<head><title>Tillandsia App</title></head>
<body>
  <h1>Hello from Tillandsia!</h1>
  <p>Deployed with Tillandsia.</p>
</body>
</html>
`

const dockerfileNode = `FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production 2>/dev/null || true
COPY . .
EXPOSE 8080
CMD ["node", "server.js"]
`

const dockerfilePython = `FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt 2>/dev/null || true
COPY . .
EXPOSE 8080
CMD ["python", "app.py"]
`

const dockerfileGo = `FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o app .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/app .
EXPOSE 8080
CMD ["./app"]
`

const dockerfileRuby = `FROM ruby:3.3-alpine
WORKDIR /app
COPY Gemfile Gemfile.lock ./
RUN bundle install 2>/dev/null || true
COPY . .
EXPOSE 8080
CMD ["ruby", "app.rb"]
`

const dockerfileStatic = `FROM nginx:alpine
COPY . /usr/share/nginx/html
EXPOSE 80
`
