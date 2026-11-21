package server

import (
	"context"
	"net/http"
)

func (s *Server) registerRadarRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/radar/capabilities", s.authCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.writeJSON(w, http.StatusOK, s.radar.Capabilities())
	})))
	mux.Handle("POST /api/radar/research", contextJSONAction(s, http.StatusOK, s.radar.Research))
	mux.Handle("POST /api/radar/receipt", contextJSONAction(s, http.StatusCreated, s.radar.Apply))
	mux.Handle("POST /api/radar/draft/plan", contextJSONAction(s, http.StatusOK, s.radar.PlanDraft))
	mux.Handle("POST /api/radar/draft/apply", contextJSONAction(s, http.StatusCreated, s.radar.CreateDraft))
}

func contextJSONAction[Req any](s *Server, status int, fn func(context.Context, Req) (map[string]any, error)) http.Handler {
	return s.authCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Req
		if err := decodeJSONBody(r, &req); err != nil {
			s.writeError(w, err)
			return
		}
		res, err := fn(r.Context(), req)
		if err != nil {
			s.writeError(w, err)
			return
		}
		s.writeJSON(w, status, res)
	}))
}
