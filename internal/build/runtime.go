package build

import (
	"fmt"
	"strings"
)

type builderTemplate struct {
	runtime string
	image   string
	gen     func(appDir, cmd string) string
}

var builderRegistry = []builderTemplate{
	{":20", "node:20-alpine", dockerfileNode},
	{":22", "node:22-alpine", dockerfileNode},
	{"node", "node:20-alpine", dockerfileNode},
	{":3.12", "python:3.12-slim", dockerfilePython},
	{":3.13", "python:3.13-slim", dockerfilePython},
	{"python", "python:3.12-slim", dockerfilePython},
	{":1.22", "golang:1.22-alpine", dockerfileGo},
	{":1.23", "golang:1.23-alpine", dockerfileGo},
	{"go", "golang:1.22-alpine", dockerfileGo},
	{":3.3", "ruby:3.3-alpine", dockerfileRuby},
	{":3.4", "ruby:3.4-alpine", dockerfileRuby},
	{"ruby", "ruby:3.3-alpine", dockerfileRuby},
	{"static", "nginx:alpine", dockerfileStatic},
}

func GenerateDockerfile(runtime, appDir, cmd string) (string, error) {
	runtime = strings.TrimSpace(runtime)

	parts := strings.SplitN(runtime, ":", 2)
	lang := parts[0]
	version := ""
	if len(parts) > 1 {
		version = ":" + parts[1]
	}

	for _, t := range builderRegistry {
		if t.runtime == runtime {
			return t.gen(appDir, cmd), nil
		}
	}

	if version != "" {
		for _, t := range builderRegistry {
			if t.runtime == version {
				return t.gen(appDir, cmd), nil
			}
		}
	}

	for _, t := range builderRegistry {
		if t.runtime == lang {
			return t.gen(appDir, cmd), nil
		}
	}

	return "", fmt.Errorf("unsupported runtime %q", runtime)
}

func SupportedRuntimes() []string {
	seen := make(map[string]bool)
	var result []string
	for _, t := range builderRegistry {
		if !seen[t.runtime] && !strings.HasPrefix(t.runtime, ":") {
			seen[t.runtime] = true
			result = append(result, t.runtime)
		}
	}
	return result
}

func dockerfileNode(appDir, cmd string) string {
	return fmt.Sprintf(`FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production 2>/dev/null || true
COPY . .
EXPOSE 8080
CMD %s
`, quoteCmd(cmd))
}

func dockerfilePython(appDir, cmd string) string {
	return fmt.Sprintf(`FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt 2>/dev/null || true
COPY . .
EXPOSE 8080
CMD %s
`, quoteCmd(cmd))
}

func dockerfileGo(appDir, cmd string) string {
	return `FROM golang:1.22-alpine AS builder
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
}

func dockerfileRuby(appDir, cmd string) string {
	return fmt.Sprintf(`FROM ruby:3.3-alpine
WORKDIR /app
COPY Gemfile Gemfile.lock ./
RUN bundle install 2>/dev/null || true
COPY . .
EXPOSE 8080
CMD %s
`, quoteCmd(cmd))
}

func dockerfileStatic(appDir, cmd string) string {
	return `FROM nginx:alpine
COPY . /usr/share/nginx/html
EXPOSE 80
`
}

func quoteCmd(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "[]"
	}
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = fmt.Sprintf("%q", p)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}