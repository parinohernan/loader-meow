package main

import "strings"

// ArgentineLocationExceptions ciudades argentinas cuyo nombre contiene palabras de países vecinos.
var ArgentineLocationExceptions = []string{
	"concepción del uruguay",
	"concepcion del uruguay",
	"chilecito",
	"perúgorría",
	"perugorria",
	"perugorría",
}

// stripLocationExceptions reemplaza frases de excepción para evitar falsos positivos en blacklist.
func stripLocationExceptions(content string) string {
	normalized := normalizeForMatching(content)
	for _, exc := range ArgentineLocationExceptions {
		normalized = strings.ReplaceAll(normalized, normalizeForMatching(exc), " ")
	}
	return normalized
}

func normalizeForMatching(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	)
	return replacer.Replace(s)
}
