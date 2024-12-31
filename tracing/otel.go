package tracing

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/go-akka/configuration"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	log "github.com/sirupsen/logrus"
)

type providerConstructor func() (oteltrace.TracerProvider, error)

var (
	providerBuilders = map[string]providerConstructor{
		"http": otelHttpTraceProvider,
		"stdout": otelStdoutTraceProvider,
		"noop": otelNoopTraceProvider,
	}
)

// defaultOtelTracerProvider is used when no provider is given.
// The Noop tracer provider turns all tracing related operations into
// noops essentially disabling tracing.
const defaultOtelTracerProvider = "noop"

// initOtel - initialize OpenTelemetry constructs
// tracing instrumentation code.
func otelInit(conf *configuration.Config) {
	xpcTracer.OtelEnabled = conf.GetBoolean("webconfig.tracing.otel.enabled")
	if !xpcTracer.OtelEnabled {
		return
	}
	xpcTracer.otelProvider = strings.ToLower(conf.GetString("webconfig.tracing.otel.provider", defaultOtelTracerProvider))
	if xpcTracer.otelProvider == "" {
		xpcTracer.otelProvider = defaultOtelTracerProvider
	}
	if xpcTracer.otelProvider == defaultOtelTracerProvider {
		log.Debug("otel disabled, noop provider")
		return
	}

	log.Debug("otel enabled")
	xpcTracer.otelEndpoint = conf.GetString("webconfig.tracing.otel.endpoint")
	xpcTracer.otelOpName = conf.GetString("webconfig.tracing.otel.operation_name")

	if providerBuilder := providerBuilders[xpcTracer.otelProvider]; providerBuilder == nil {
		log.Errorf("no builder func for otel provider %s", xpcTracer.otelProvider)
		return
	} else {
		var err error
		if xpcTracer.otelTracerProvider, err = providerBuilder(); err != nil {
			log.Errorf("building otel provider for %s failed with %v", xpcTracer.otelProvider, err)
			return
		}
	}
	otel.SetTracerProvider(xpcTracer.otelTracerProvider)

	// Set up propagator.
	xpcTracer.otelPropagator = propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(xpcTracer.otelPropagator)

	xpcTracer.otelTracer = otel.Tracer(xpcTracer.appName)
}

func otelNoopTraceProvider() (oteltrace.TracerProvider, error) {
	return noop.NewTracerProvider(), nil
}

func otelStdoutTraceProvider() (oteltrace.TracerProvider, error) {
	option := stdouttrace.WithPrettyPrint()
	exporter, err := stdouttrace.New(option)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter),
		sdktrace.WithBatcher(exporter,
			// Default is 5s. Set to 1s for demonstrative purposes.
			sdktrace.WithBatchTimeout(time.Second)),
		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(xpcTracer.appName),
				semconv.ServiceNamespaceKey.String(xpcTracer.appEnv),
			),
		),
	)
	return tp, nil
}

func otelHttpTraceProvider() (oteltrace.TracerProvider, error) {
	// Send traces over HTTP
	if xpcTracer.otelEndpoint == "" {
		return nil, fmt.Errorf("building http otel provider failure, no endpoint specified")
	}
	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(xpcTracer.otelEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("building http otel provider failed with %v", err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(xpcTracer.appName),
				semconv.ServiceNamespaceKey.String(xpcTracer.appEnv),
			),
		),
	), nil
}

func otelShutdown() {
	sdkTraceProvider, ok := xpcTracer.otelTracerProvider.(*sdktrace.TracerProvider)
	if ok && sdkTraceProvider != nil {
		sdkTraceProvider.Shutdown(context.TODO())
	}
}

// otelOpName should return "http.request" by default
func otelOpName() string {
	opName := xpcTracer.otelOpName
	if opName == "" {
		opName = "http.request"
	}
	return opName
}

func otelNewSpan(r *http.Request) (context.Context, oteltrace.Span) {
	ctx := r.Context()
	var otelSpan oteltrace.Span
	if !xpcTracer.OtelEnabled {
		return ctx, otelSpan
	}

	pathTemplate := "placeholder"
	if mux.CurrentRoute(r) != nil { // This can be nil in unit tests
		var err error
		pathTemplate, err = mux.CurrentRoute(r).GetPathTemplate()
		if err != nil {
			log.Debugf("unable to get path template: %v", err)
		}
	}
	resourceName := r.Method + " " + pathTemplate
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))

	/*
		required span attribute:  HTTPMethodKey = attribute.Key("http.method")
		required span attribute:  HTTPRouteKey = attribute.Key("http.route")
		required span attribute:  HTTPStatusCodeKey = attribute.Key("http.status_code")
		required span attribute:  HTTPURLKey = attribute.Key("http.url")
		custom Comcast attribute: X-Cl-Experiment: true/false
		additional: env, operation.name, http.url_details.path
	*/
	ctx, otelSpan = xpcTracer.otelTracer.Start(ctx, otelOpName(),
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			attribute.String("env", xpcTracer.appEnv),
			attribute.String("http.method", r.Method),
			attribute.String("http.route", pathTemplate),
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.url_details.path", r.URL.Path),
			attribute.String("operation.name", otelOpName()),
		),
	)
	if xpcTracer.rgn != "" {
		rgnAttr := attribute.String("region", xpcTracer.rgn)
		otelSpan.SetAttributes(rgnAttr)
	}

	log.Debugf("span started %s", resourceName)
	log.Debugf("added span attribute key = env, value = %s", xpcTracer.appEnv)
	log.Debugf("added span attribute key = http.method, value = %s", r.Method)
	log.Debugf("added span attribute key = http.route, value = %s", pathTemplate)
	log.Debugf("added span attribute key = http.url, value = %s", r.URL.String())
	log.Debugf("added span attribute key = http.url_details.path, value = %s", r.URL.Path)
	log.Debugf("added span attribute key = operation.name, value = %s", otelOpName())

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	for key, val := range carrier {
		ctx = SetContext(ctx, "otel_"+key, val)
		log.Debugf("OtelSpanCreation: otel %s = %s", key, val)
	}

	ctx = SetContext(ctx, "otel_span", otelSpan)
	return ctx, otelSpan
}

func otelSetStatusCode(span oteltrace.Span, statusCode int) {
	statusAttr := attribute.Int("http.status_code", statusCode)
	span.SetAttributes(statusAttr)
	log.Debugf("added span attribute key = http.status_code, value = %d", statusCode)

	if statusCode >= http.StatusInternalServerError {
		statusText := http.StatusText(statusCode)
		span.SetStatus(codes.Error, statusText)
		span.SetAttributes(attribute.String("http.response.error", statusText))
		log.Debugf("added span attribute key=http.response.error, value=%s", statusText)
	}
}

func otelEndSpan(span oteltrace.Span) {
	if !xpcTracer.OtelEnabled {
		return
	}
	span.End()
}

func otelExtractParamsFromSpan(ctx context.Context, xpcTrace *XpcTrace) {
	if !xpcTracer.OtelEnabled {
		return
	}
	if tmp := GetContext(ctx, "otel_span"); tmp != nil {
		if otelSpan, ok := tmp.(oteltrace.Span); ok {
			xpcTrace.otelSpan = otelSpan
		}
	}
	if xpcTrace.otelSpan == nil {
		return
	}
	// if otel span is found, use the extracted traceparent and tracestate from the otel span
	// We store the extracted values in ctx when we created the otel span
	if tmp := GetContext(ctx, "otel_traceparent"); tmp != nil {
		xpcTrace.otelTraceparent = tmp.(string)
		log.Debugf("Tracing: otel traceparent = %s", xpcTrace.otelTraceparent)
		xpcTrace.OutTraceparent = xpcTrace.otelTraceparent
	}
	if tmp := GetContext(ctx, "otel_tracestate"); tmp != nil {
		xpcTrace.otelTracestate = tmp.(string)
		log.Debugf("Tracing: otel tracestate = %s", xpcTrace.otelTracestate)
		xpcTrace.OutTracestate = xpcTrace.otelTracestate
	}
}
