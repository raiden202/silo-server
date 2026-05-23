package pluginhost

import (
	"time"

	"github.com/hashicorp/go-plugin"

	sdkruntime "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtime"
)

const (
	DefaultHealthCheckInterval = 30 * time.Second
	DefaultHealthFailureLimit  = 3
	DefaultMetadataTimeout     = 30 * time.Second
	DefaultAnalyzerTimeout     = 5 * time.Minute
	DefaultControlTimeout      = 10 * time.Second
	DefaultEventTimeout        = 10 * time.Second
	DefaultAuthTimeout         = 10 * time.Second
	DefaultRouteTimeout        = 10 * time.Second
)

func HandshakeConfig() plugin.HandshakeConfig {
	return sdkruntime.HandshakeConfig()
}
