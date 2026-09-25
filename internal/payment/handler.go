package payment

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Handler handles HTTP requests for webhook events from payment gateways.
type Handler struct {
	service *Service
}

// NewHandler constructs a new payment Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// HandleWebhook processes an incoming webhook callback from a specified payment provider.
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	log.Printf("[Webhook] Incoming notification for provider: %q from %s", providerName, r.RemoteAddr)

	if providerName == "" {
		http.Error(w, "missing provider", http.StatusBadRequest)
		return
	}

	if err := h.service.ProcessWebhook(r.Context(), providerName, r); err != nil {
		log.Printf("[Webhook ERROR] Failed processing %s notification: %v", providerName, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[Webhook SUCCESS] Successfully processed and fulfilled %s notification", providerName)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
