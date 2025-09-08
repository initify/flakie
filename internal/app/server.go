package app

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	gh "github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

type Config struct {
	AppID         int64
	PrivateKeyPEM []byte
	WebhookSecret string
}

func LoadConfigFromEnv() (*Config, error) {
	appIDStr := os.Getenv("FLAKIE_APP_ID")
	key := os.Getenv("FLAKIE_PRIVATE_KEY")
	secret := os.Getenv("FLAKIE_WEBHOOK_SECRET")
	if appIDStr == "" || key == "" || secret == "" {
		return nil, fmt.Errorf("missing FLAKIE_APP_ID or FLAKIE_PRIVATE_KEY or FLAKIE_WEBHOOK_SECRET")
	}
	var appID int64
	_, err := fmt.Sscan(appIDStr, &appID)
	if err != nil {
		return nil, err
	}
	return &Config{AppID: appID, PrivateKeyPEM: []byte(key), WebhookSecret: secret}, nil
}

type Server struct {
	cfg *Config
}

func NewServer(cfg *Config) *Server { return &Server{cfg: cfg} }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := gh.ValidatePayload(r, []byte(s.cfg.WebhookSecret))
	if err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	event, err := gh.ParseWebHook(gh.WebHookType(r), payload)
	if err != nil {
		http.Error(w, "bad event", http.StatusBadRequest)
		return
	}
	switch e := event.(type) {
	case *gh.PullRequestEvent:
		action := e.GetAction()
		if action == "opened" || action == "reopened" || action == "synchronize" || action == "ready_for_review" {
			go s.handlePREvent(r.Context(), e)
		}
	default:
		// ignore other events
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handlePREvent(ctx context.Context, e *gh.PullRequestEvent) {
	owner := e.GetRepo().GetOwner().GetLogin()
	repo := e.GetRepo().GetName()
	prNum := e.GetNumber()
	sha := e.GetPullRequest().GetHead().GetSHA()

	log.Printf("handling PR %s/%s#%d @ %s", owner, repo, prNum, sha)

	cli, instID, tokenStr, err := s.installationClient(ctx, owner, repo)
	if err != nil {
		log.Printf("auth error: %v", err)
		return
	}

	// Post a pending comment
	body := "🧪 Flakie bot: running flaky test detection..."
	_, _, _ = cli.Issues.CreateComment(ctx, owner, repo, prNum, &gh.IssueComment{Body: &body})

	// Download tarball for the PR head
	tmp, err := os.MkdirTemp("", "flakie-pr-*")
	if err != nil {
		log.Printf("tmp err: %v", err)
		return
	}
	defer os.RemoveAll(tmp)

	if err := downloadAndExtractTarball(ctx, tokenStr, owner, repo, sha, tmp); err != nil {
		log.Printf("tarball err: %v", err)
		failMsg := fmt.Sprintf("Flakie bot: setup failed: %v", err)
		_, _, _ = cli.Issues.CreateComment(ctx, owner, repo, prNum, &gh.IssueComment{Body: &failMsg})
		return
	}

	// Run flakiness checks internally
	// Workdir is the extracted tarball top-level directory (single child folder)
	globs, _ := filepath.Glob(filepath.Join(tmp, "*"))
	workdir := tmp
	if len(globs) > 0 {
		workdir = globs[0]
	}

	summary, comment := runFlakiness(ctx, workdir, 5, "./...", 10*time.Minute, false, "")
	_ = summary // future: attach as artifact somewhere
	_, _, _ = cli.Issues.CreateComment(ctx, owner, repo, prNum, &gh.IssueComment{Body: &comment})

	_ = instID
}

// runFlakiness runs `go test` repeatedly in workdir and returns a summary and a human-readable comment.
func runFlakiness(ctx context.Context, workdir string, runs int, pkg string, timeout time.Duration, race bool, extraArgs string) (map[string]map[string]int, string) {
	reRun := regexp.MustCompile(`^=== RUN\s+([A-Za-z0-9_\-/]+)`)
	rePass := regexp.MustCompile(`^--- PASS: \s*([A-Za-z0-9_\-/]+)`)
	reFail := regexp.MustCompile(`^--- FAIL: \s*([A-Za-z0-9_\-/]+)`)

	stats := map[string]map[string]int{}
	for i := 0; i < runs; i++ {
		perTimeout, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		args := []string{"test", pkg, "-count=1", "-run", ".", "-v"}
		if race {
			args = append(args, "-race")
		}
		if strings.TrimSpace(extraArgs) != "" {
			args = append(args, strings.Fields(extraArgs)...)
		}
		cmd := exec.CommandContext(perTimeout, "go", args...)
		cmd.Dir = workdir
		out, _ := cmd.CombinedOutput()
		scanner := bufio.NewScanner(bytes.NewReader(out))
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if m := reRun.FindStringSubmatch(line); m != nil {
				t := m[1]
				if _, ok := stats[t]; !ok {
					stats[t] = map[string]int{"pass": 0, "fail": 0}
				}
				continue
			}
			if m := rePass.FindStringSubmatch(line); m != nil {
				t := m[1]
				if _, ok := stats[t]; !ok {
					stats[t] = map[string]int{"pass": 0, "fail": 0}
				}
				stats[t]["pass"]++
				continue
			}
			if m := reFail.FindStringSubmatch(line); m != nil {
				t := m[1]
				if _, ok := stats[t]; !ok {
					stats[t] = map[string]int{"pass": 0, "fail": 0}
				}
				stats[t]["fail"]++
				continue
			}
		}
	}
	// Build comment
	var flaky []string
	var failed []string
	for t, c := range stats {
		if c["pass"] > 0 && c["fail"] > 0 {
			flaky = append(flaky, fmt.Sprintf("%s (pass=%d, fail=%d)", t, c["pass"], c["fail"]))
		}
		if c["pass"] == 0 && c["fail"] > 0 {
			failed = append(failed, fmt.Sprintf("%s (fail=%d)", t, c["fail"]))
		}
	}
	var b strings.Builder
	b.WriteString("🧪 Flakie bot report\n\n")
	if len(flaky) == 0 && len(failed) == 0 {
		b.WriteString(fmt.Sprintf("No flaky tests detected after %d runs.\n", runs))
	} else {
		if len(flaky) > 0 {
			b.WriteString("Flaky tests detected:\n")
			for _, l := range flaky {
				b.WriteString("- ")
				b.WriteString(l)
				b.WriteString("\n")
			}
		} else {
			b.WriteString("No flaky tests detected.\n")
		}
		if len(failed) > 0 {
			b.WriteString("\nConsistently failing tests:\n")
			for _, l := range failed {
				b.WriteString("- ")
				b.WriteString(l)
				b.WriteString("\n")
			}
		}
	}
	return stats, b.String()
}

func (s *Server) installationClient(ctx context.Context, owner, repo string) (*gh.Client, int64, string, error) {
	// Build app JWT
	key, err := parsePrivateKey(s.cfg.PrivateKeyPEM)
	if err != nil {
		return nil, 0, "", err
	}

	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": s.cfg.AppID,
	})
	signed, err := token.SignedString(key)
	if err != nil {
		return nil, 0, "", err
	}

	// App-level client using JWT (Bearer)
	appTS := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: signed, TokenType: "Bearer"})
	appClient := gh.NewClient(oauth2.NewClient(ctx, appTS))

	inst, _, err := appClient.Apps.FindRepositoryInstallation(ctx, owner, repo)
	if err != nil {
		return nil, 0, "", err
	}

	// Create installation token
	tok, _, err := appClient.Apps.CreateInstallationToken(ctx, inst.GetID(), &gh.InstallationTokenOptions{})
	if err != nil {
		return nil, 0, "", err
	}

	// Client with installation token
	instTS := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok.GetToken()})
	return gh.NewClient(oauth2.NewClient(ctx, instTS)), inst.GetID(), tok.GetToken(), nil
}

func parsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("pem decode failed")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return key, nil
	}
	k2, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err2 != nil {
		return nil, err
	}
	rk, ok := k2.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not rsa key")
	}
	return rk, nil
}

func downloadAndExtractTarball(ctx context.Context, token, owner, repo, ref, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s", owner, repo, ref), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	httpClient := &http.Client{Timeout: 2 * time.Minute}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("tarball download status: %s", resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}
