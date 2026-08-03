package main

import (
	"testing"

	"github.com/muzaffer/internship-tracker/internal/config"
)

func TestConfigureAnalyzerSelectsProviderAndValidatesOpenRouterSettings(t *testing.T) {
	deterministic, err := configureAnalyzer(config.Config{LLMProvider: "deterministic"})
	if err != nil || deterministic == nil {
		t.Fatalf("configure deterministic analyzer: analyzer=%#v err=%v", deterministic, err)
	}

	_, err = configureAnalyzer(config.Config{LLMProvider: "openrouter", LLMModel: "model"})
	if err == nil {
		t.Fatal("OpenRouter must require an API key")
	}

	configured, err := configureAnalyzer(config.Config{
		LLMProvider: "openrouter", LLMModel: "provider/model", OpenRouterAPIKey: "secret",
		LLMInputCost: "0.1", LLMOutputCost: "0.2",
	})
	if err != nil || configured == nil {
		t.Fatalf("configure OpenRouter analyzer: analyzer=%#v err=%v", configured, err)
	}
}

func TestAnalyzerProfileRemovesInstitutionNames(t *testing.T) {
	profile := analyzerProfile(config.CandidateProfile{
		Education:  config.EducationProfile{University: "Private University", Department: "CTIS", ClassYear: 2},
		Experience: []config.ExperienceProfile{{Organization: "Private Company", Areas: []string{"backend"}}},
	})
	if profile.EducationField != "CTIS" || len(profile.ExperienceAreas) != 1 || profile.ExperienceAreas[0] != "backend" {
		t.Fatalf("required profile attributes were not mapped: %#v", profile)
	}
}
