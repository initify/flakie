# flakie

Flakie is a GitHub App (bot) that detects flaky tests on pull requests by running tests multiple times and commenting the results.

## GitHub App setup

1) Create a GitHub App (in your org or user):
- Webhook URL: https://your-domain/webhook
- Webhook secret: set and keep safe
- Repository permissions: Pull requests (Read & write), Contents (Read)
- Subscribe to: Pull request events
- Generate and download the private key (PEM) and note the App ID

2) Install the App on your repositories (or all)

3) Run the bot server:
- Env vars required:
  - FLAKIE_APP_ID: numeric App ID
  - FLAKIE_PRIVATE_KEY: PEM contents
  - FLAKIE_WEBHOOK_SECRET: webhook secret
- Run:
  - go build -o flakie-bot ./cmd/bot
  - PORT=8080 FLAKIE_APP_ID=... FLAKIE_PRIVATE_KEY="$(cat private-key.pem)" FLAKIE_WEBHOOK_SECRET=... ./flakie-bot

Expose the server publicly (ngrok/cloudflared/your infra) and set the Webhook URL accordingly.

## What it does

- Listens for pull_request events
- Downloads the PR head tarball
- Runs `go test` multiple times to detect flaky tests
- Posts a PR comment summarizing flaky and consistently failing tests

## Examples

Sample packages under `examples/` include stable tests and an intentionally flaky test you can use to try the bot locally.
