// backend/internal/services/billingo_errors.go
package services

import (
	"log"
	"strings"
)

// FriendlyBillingoError logs the raw Billingo/config error (for debugging)
// and returns a short Hungarian message safe to show to end users, since the
// raw error (Go wrapping + a raw Billingo JSON validation payload) is not
// meant for non-technical readers. Shared by the manual invoice-creation
// handler and the automatic monthly invoicing job.
func FriendlyBillingoError(err error) string {
	log.Printf("⚠️ Billingo invoice error: %v", err)

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "api key is not configured") || strings.Contains(msg, "settings not configured"):
		return "A Billingo API kulcs nincs beállítva. Állítsa be a Rendszerbeállítások / Billingo menüpontban."
	case strings.Contains(msg, "block id"):
		return "Érvénytelen Billingo blokkazonosító van beállítva. Ellenőrizze a Billingo beállításokat."
	case strings.Contains(msg, "partner id"):
		return "Hiba történt az ügyfél Billingo partnerként való létrehozásakor."
	case strings.Contains(msg, "returned 401") || strings.Contains(msg, "returned 403"):
		return "A Billingo elutasította az API kulcsot. Ellenőrizze a Billingo beállításokat."
	case strings.Contains(msg, "returned 422"):
		return "A Billingo hiányos vagy hibás számlaadatokat kapott. Ellenőrizze a Billingo beállításokat (mértékegység, egységár típusa)."
	case strings.Contains(msg, "calling billingo"):
		return "Nem sikerült elérni a Billingo szolgáltatást. Próbálja meg később."
	default:
		return "Hiba történt a Billingo számla létrehozásakor. Próbálja újra, vagy ellenőrizze a Billingo beállításokat."
	}
}
