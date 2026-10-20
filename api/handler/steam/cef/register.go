package cef

import (
	"github.com/Alia5/SISR/cmd"
	"github.com/danielgtaylor/huma/v2"
)

func Register(a huma.API, c *cmd.SISRContext) {
	registerDiagnostics(a)

	registerOverlayStateChanged(a, c)
	registerVirtualMenuVisibility(a, c)
	registerInjectOverlayNotifier(a, c)
	registerCreateMarkerShortcut(a, c)
	registerOpenConfigurator(a, c)

}
