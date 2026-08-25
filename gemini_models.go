package main

import "strings"

// normalizeGeminiModelName reemplaza IDs de modelos deprecados por equivalentes vigentes en la API.
func normalizeGeminiModelName(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "gemini-2.5-flash"
	}

	aliases := map[string]string{
		"gemini-1.5-flash-latest":    "gemini-2.5-flash",
		"gemini-1.5-flash":           "gemini-2.5-flash",
		"gemini-1.5-flash-8b":        "gemini-2.5-flash-lite",
		"gemini-1.5-flash-8b-latest": "gemini-2.5-flash-lite",
		"gemini-1.5-pro":             "gemini-2.5-pro",
		"gemini-1.5-pro-latest":      "gemini-2.5-pro",
		"gemini-2.0-flash-exp":       "gemini-2.5-flash",
		"gemini-2.0-flash":           "gemini-2.5-flash",
		"gemini-2.0-flash-001":       "gemini-2.5-flash",
		"gemini-2.0-flash-lite":      "gemini-2.5-flash-lite",
		"gemini-2.0-flash-lite-001":  "gemini-2.5-flash-lite",
	}

	if mapped, ok := aliases[model]; ok {
		return mapped
	}
	return model
}
