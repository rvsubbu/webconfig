package tracing

import (
	"strings"
	"os"

	"github.com/go-akka/configuration"

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

	// internal vars for homegrown trace generation/modification
	appID                     string // used in the homegrown traceparent,tracestate header generation
	homegrownTracePropagation bool // Use homegrown algorithm for traceparent, tracestate modification
	homegrownTraceGeneration  bool // Generate traceparent etc. using homegrown algorithm if not present in incoming req
}

var xpcTracer XpcTracer // global tracer

func NewXpcTracer(conf *configuration.Config) *XpcTracer {
	initAppData(conf)
	initHomegrownTracing(conf)

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
	siteColor := os.Getenv("SITE_COLOR")
	if strings.EqualFold(siteColor, "yellow") {
		xpcTracer.appEnv = "staging"
	} else if strings.EqualFold(siteColor, "green") {
		xpcTracer.appEnv = "prod"
	}
	xpcTracer.rgn = os.Getenv("SITE_REGION")
}

func initHomegrownTracing(conf *configuration.Config) {
	// TODO: Marshal these tracing params from config, ok to temporarily read each separately
	xpcTracer.appID = conf.GetString("webconfig.tracing.homegrown_algorithm.app_id", defaultAppID)
	xpcTracer.homegrownTracePropagation = conf.GetBoolean("webconfig.tracing.homegrown_algorithm.xpc_trace_propagation")
	xpcTracer.homegrownTraceGeneration = conf.GetBoolean("webconfig.tracing.homegrown_algorithm.xpc_trace_generation")
}

func GetServiceName() string {
	return xpcTracer.appName
}
