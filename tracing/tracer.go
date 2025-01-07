package tracing

import (
	"strings"
	"os"

	"github.com/go-akka/configuration"
	log "github.com/sirupsen/logrus"

	oteltrace "go.opentelemetry.io/otel/trace"
	otelpropagation "go.opentelemetry.io/otel/propagation"
)

const (
	AuditIDHeader = "X-Auditid"
	UserAgentHeader = "User-Agent"

	defaultMoracideTagPrefix = "X-Cl-Experiment"
)

// XpcTracer is a wrapper around tracer setup
type XpcTracer struct {
	OtelEnabled       bool
	MoracideTagPrefix string // Special request header for moracide expts e.g. canary deployments

	// internal vars used by Otel
	appEnv     string // set this to dev for red, staging for yellow and prod for green
	appName    string
	appVersion string
	appSHA     string // unused
	rgn        string // AWS Region e.g. us-west-2, unused, use it as a otel span attribute
	siteColor  string // red/yellow/green, unused, use it as a otel span attribute

	// internal otel vars
	otelEndpoint       string
	otelOpName         string
	otelProvider       string
	otelTracerProvider oteltrace.TracerProvider
	otelPropagator     otelpropagation.TextMapPropagator
	otelTracer         oteltrace.Tracer
}

var xpcTracer XpcTracer // global tracer

func NewXpcTracer(conf *configuration.Config) *XpcTracer {
	initAppData(conf)
	otelInit(conf)
	xpcTracer.MoracideTagPrefix = conf.GetString("webconfig.tracing.moracide_tag_prefix", defaultMoracideTagPrefix)
	return &xpcTracer
}

// defer this func in the main of the app
func StopXpcTracer() {
	otelShutdown()
}

// Global func to access the moracide tag
func GetMoracideTagPrefix() string {
	return xpcTracer.MoracideTagPrefix
}

func initAppData(conf *configuration.Config) {
	codeGitCommit := strings.Split(conf.GetString("webconfig.code_git_commit"), "-")
	xpcTracer.appName = codeGitCommit[0]
	if len(codeGitCommit) > 1 {
		xpcTracer.appVersion = codeGitCommit[1]
	}
	if len(codeGitCommit) > 2 {
		xpcTracer.appSHA = codeGitCommit[2]
	}

	// Env vars
	xpcTracer.appEnv = "dev"
	siteColor := os.Getenv("site_color")
	if strings.EqualFold(siteColor, "yellow") {
		xpcTracer.appEnv = "staging"
	} else if strings.EqualFold(siteColor, "green") {
		xpcTracer.appEnv = "prod"
	}
	xpcTracer.rgn = os.Getenv("site_region")
	if xpcTracer.rgn == "" {
		xpcTracer.rgn = os.Getenv("site_region_name")
	}
	log.Debugf("site_color = %s, env = %s, region = %s", siteColor, xpcTracer.appEnv, xpcTracer.rgn)
}

func GetServiceName() string {
	return xpcTracer.appName
}
