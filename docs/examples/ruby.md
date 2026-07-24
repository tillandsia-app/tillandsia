# Ruby Quickstart

Deploy a Ruby application with Tillandsia.

## 1. Create Your Project

```bash
mkdir my-app && cd my-app
```

Create `Gemfile`:

```ruby
source 'https://rubygems.org'
gem 'sinatra'
gem 'puma'
```

Create `app.rb`:

```ruby
require 'sinatra'

port = ENV['PORT'] || 8080
set :port, port
set :bind, '0.0.0.0'

get '/' do
  'Hello from Tillandsia!'
end
```

Install dependencies:

```bash
bundle install
```

## 2. Initialize Tillandsia

```bash
tillandsia init
```

This detects `Gemfile` and generates:
- `Dockerfile` — Ruby 3.2 slim image
- `Procfile` — `web: bundle exec ruby app.rb`
- `tillandsia.yaml` — Project configuration

## 3. Add a Server

```bash
tillandsia server add my-vps --host <your-vps-ip>
tillandsia server setup my-vps
```

## 4. Deploy

```bash
tillandsia deploy
```

## 5. Verify

```bash
tillandsia logs
tillandsia status
```

Your app is running at `http://<your-vps-ip>:80`.