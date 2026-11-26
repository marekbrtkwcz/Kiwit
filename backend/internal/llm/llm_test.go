package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseScenarios(t *testing.T) {
	g := `Feature: Login
  Scenario: Valid credentials
    Given user is on /login
    When user submits valid credentials
    Then user sees dashboard
  Scenario: Wrong password
    Given user is on /login
    When user submits wrong password
    Then error is shown
    And user stays on /login`
	scs, err := ParseScenarios(g)
	if err != nil {
		t.Fatal(err)
	}
	if len(scs) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(scs))
	}
	if scs[0].Name != "Valid credentials" || len(scs[0].Steps) != 3 {
		t.Fatalf("unexpected scenario 0: %+v", scs[0])
	}
	if len(scs[1].Steps) != 4 {
		t.Fatalf("expected 4 steps in scenario 1, got %d", len(scs[1].Steps))
	}
}

func TestParseScenariosNoScenarios(t *testing.T) {
	if _, err := ParseScenarios("just some prose"); err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractGherkin(t *testing.T) {
	raw := "Here you go:\n```gherkin\nFeature: X\n  Scenario: S\n    Given a\n```\nThanks!"
	g := ExtractGherkin(raw)
	if g == "" || g[0] != 'F' {
		t.Fatalf("expected gherkin block, got %q", g)
	}
}

func TestChatMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req chatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "test-model" {
			t.Errorf("unexpected model %q", req.Model)
		}
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "```gherkin\nFeature: F\n  Scenario: S\n    Given x\n```"}}},
		})
	}))
	defer srv.Close()
	cfg := Config{APIKey: "test-key", BaseURL: srv.URL, Model: "test-model"}
	out, err := cfg.Chat(context.Background(), "sys", "user")
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("empty response")
	}
	scs, err := ParseScenarios(ExtractGherkin(out))
	if err != nil || len(scs) != 1 {
		t.Fatalf("expected 1 scenario, got %v %v", scs, err)
	}
}

func TestChatDisabled(t *testing.T) {
	cfg := Config{}
	if cfg.Enabled() {
		t.Fatal("expected disabled")
	}
	if _, err := cfg.Chat(context.Background(), "s", "u"); err == nil {
		t.Fatal("expected error when disabled")
	}
}
