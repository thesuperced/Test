package cef

import (
	"context"
	"net/http"

	"github.com/Alia5/SISR/logging"
	"github.com/danielgtaylor/huma/v2"
)

type diagnosticInput struct {
	Body struct {
		Kind        string `json:"kind" enum:"injection,menu_message,menu_lifecycle,overlay"`
		Controller  int    `json:"controller,omitempty"`
		Menu        int    `json:"menu,omitempty"`
		Created     bool   `json:"created,omitempty"`
		Open        bool   `json:"open,omitempty"`
		Parsed      bool   `json:"parsed,omitempty"`
		MessageType string `json:"messageType,omitempty" maxLength:"40"`
		Length      int    `json:"length,omitempty"`
	}
}

func registerDiagnostics(a huma.API) {
	huma.Register(a, huma.Operation{
		Method: http.MethodPost, Path: "/api/v1/steam/cef/diagnostics",
		Tags: []string{"steam", "cef", "callback"},
	}, func(ctx context.Context, req *diagnosticInput) (*struct{}, error) {
		b := req.Body
		logging.Diagnostic("cef_"+b.Kind, map[string]any{
			"controller": b.Controller, "menu": b.Menu, "created": b.Created,
			"open": b.Open, "parsed": b.Parsed, "message_type": b.MessageType, "length": b.Length,
		})
		return nil, nil
	})
}
