package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// SessionDTO is the JSON body ServeSession writes: whether authentication
// is in force, who is calling, whether they may write, and where they sign
// in or out.
type SessionDTO struct {
	Enabled       bool     `json:"enabled"`
	Authenticated bool     `json:"authenticated"`
	UserId        string   `json:"userId,omitempty"`
	Name          string   `json:"name,omitempty"`
	Groups        []string `json:"groups"`
	CanWrite      bool     `json:"canWrite"`
	LoginUrl      string   `json:"loginUrl,omitempty"`
	LogoutUrl     string   `json:"logoutUrl,omitempty"`
}

// ServeSession answers with the current caller, so a browser application
// can render its own state instead of guessing it from failed writes. It is
// a read: an anonymous request gets an answer, not a refusal, which is what
// lets a client offer a sign-in. Mount it wherever the application's routes
// live — the path is the only part of this endpoint the application still
// owns.
//
// @Summary Current caller
// @Description Whether authentication is in force, who the caller is, whether they may write, and the gateway's sign-in/sign-out URLs. Answers an anonymous request.
// @Tags auth
// @Produce json
// @Success 200 {object} auth.SessionDTO
// @Router /api/v1/session [get]
func (a *Authenticator) ServeSession(w http.ResponseWriter, req *http.Request) {
	session := a.Session(req.Context())

	groups := session.User.Groups
	if groups == nil {
		groups = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(SessionDTO{
		Enabled:       session.Enabled,
		Authenticated: session.Authenticated,
		UserId:        session.User.ID,
		Name:          session.User.Name,
		Groups:        groups,
		CanWrite:      session.CanWrite,
		LoginUrl:      session.LoginURL,
		LogoutUrl:     session.LogoutURL,
	}); err != nil {
		slog.Error("encoding session response", "error", err)
	}
}
