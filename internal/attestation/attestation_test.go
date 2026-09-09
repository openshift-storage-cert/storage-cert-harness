package attestation

import (
	"context"
	"strings"
	"testing"
)

func TestLoadQuestionsDefault(t *testing.T) {
	qs, err := LoadQuestions("")
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) == 0 {
		t.Fatal("expected bundled default questions")
	}
	for _, q := range qs {
		if q.Claim == "" || q.Prompt == "" {
			t.Fatalf("malformed default question: %+v", q)
		}
	}
}

func TestPromptDeclinedGate(t *testing.T) {
	qs, _ := LoadQuestions("")
	atts, err := Prompt(context.Background(), qs, strings.NewReader("n\n"), &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if atts != nil {
		t.Fatalf("declining the gate must yield no attestations, got %+v", atts)
	}
}

func TestPromptCollectsAnswers(t *testing.T) {
	qs := []Question{
		{Claim: "ra_url", Prompt: "RA URL", Required: true},
		{Claim: "notes", Prompt: "Notes", Required: false},
	}
	// gate=y, signer, required answer, then blank-skip the optional one.
	in := strings.NewReader("y\nalice@example.com\nhttps://ra\n\n")
	atts, err := Prompt(context.Background(), qs, in, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if len(atts) != 1 {
		t.Fatalf("want 1 attestation (optional skipped), got %d: %+v", len(atts), atts)
	}
	if atts[0].Claim != "ra_url" || atts[0].Value != "https://ra" || atts[0].SignedBy != "alice@example.com" {
		t.Fatalf("unexpected attestation: %+v", atts[0])
	}
}

func TestPromptReAsksRequired(t *testing.T) {
	qs := []Question{{Claim: "ra_url", Prompt: "RA URL", Required: true}}
	// gate=y, signer, blank (rejected), then a value.
	in := strings.NewReader("yes\nme\n\nhttps://ra\n")
	atts, err := Prompt(context.Background(), qs, in, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if len(atts) != 1 || atts[0].Value != "https://ra" {
		t.Fatalf("required question should be re-asked until answered: %+v", atts)
	}
}
