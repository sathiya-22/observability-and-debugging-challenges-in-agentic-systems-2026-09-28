package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"agent-observability-prototype/agent_observability"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"google.golang.org/grpc"
)

const (
	port = ":50051"
)

// AgentObservabilityServer implements the gRPC service
type AgentObservabilityServer struct {
	agent_observability.UnimplementedAgentObservabilityServiceServer
	tracer trace.Tracer
}

func NewAgentObservabilityServer(tracer trace.Tracer) *AgentObservabilityServer {
	return &AgentObservabilityServer{tracer: tracer}
}

func (s *AgentObservabilityServer) SendEvent(ctx context.Context, event *agent_observability.AgentEvent) (*agent_observability.SendEventResponse, error) {
	// Create a new span for each event, linking it to the session if possible.
	// In a real system, you'd manage span contexts more robustly, possibly
	// by passing parent span IDs in the event itself or using a context propagator.
	// For this prototype, we'll create a new root span for each event for simplicity,
	// or create a child span if a parent context is available.

	_, span := s.tracer.Start(ctx, fmt.Sprintf("AgentEvent: %s", event.GetEventType().String()))
	defer span.End()

	span.SetAttributes(
		attribute.String("event.id", event.GetEventId()),
		attribute.String("event.timestamp", event.GetTimestamp()),
		attribute.String("agent.id", event.GetAgentId()),
		attribute.String("session.id", event.GetSessionId()),
	)

	// Add event-specific details as attributes
	switch e := event.GetEventType().(type) {
	case *agent_observability.AgentEvent_AgentStart:
		span.SetName("Agent Start")
		span.SetAttributes(
			attribute.String("agent.goal", e.AgentStart.GetInitialGoal()),
			attribute.StringMap("agent.context", e.AgentStart.GetContext()),
		)
	case *agent_observability.AgentEvent_AgentDecision:
		span.SetName("Agent Decision")
		span.SetAttributes(
			attribute.String("decision.thought", e.AgentDecision.GetThought()),
			attribute.String("decision.type", e.AgentDecision.GetDecisionType()),
			attribute.StringMap("decision.parameters", e.AgentDecision.GetParameters()),
		)
	case *agent_observability.AgentEvent_ToolCall:
		span.SetName(fmt.Sprintf("Tool Call: %s", e.ToolCall.GetToolName()))
		span.SetAttributes(
			attribute.String("tool.name", e.ToolCall.GetToolName()),
			attribute.String("tool.input", e.ToolCall.GetToolInput()),
			attribute.String("tool.call_id", e.ToolCall.GetCallId()),
		)
	case *agent_observability.AgentEvent_ToolResult:
		span.SetName(fmt.Sprintf("Tool Result: %s", e.ToolResult.GetToolName()))
		span.SetAttributes(
			attribute.String("tool.name", e.ToolResult.GetToolName()),
			attribute.String("tool.output", e.ToolResult.GetToolOutput()),
			attribute.String("tool.call_id", e.ToolResult.GetCallId()),
			attribute.Bool("tool.success", e.ToolResult.GetSuccess()),
			attribute.String("tool.error", e.ToolResult.GetErrorMessage()),
		)
	case *agent_observability.AgentEvent_LlmInteraction:
		span.SetName("LLM Interaction")
		span.SetAttributes(
			attribute.String("llm.model_name", e.LlmInteraction.GetModelName()),
			attribute.String("llm.prompt", e.LlmInteraction.GetPrompt()),
			attribute.String("llm.response", e.LlmInteraction.GetResponse()),
			attribute.StringMap("llm.metadata", e.LlmInteraction.GetMetadata()),
		)
	case *agent_observability.AgentEvent_AgentEnd:
		span.SetName("Agent End")
		span.SetAttributes(
			attribute.String("agent.final_result", e.AgentEnd.GetFinalResult()),
			attribute.String("agent.status", e.AgentEnd.GetStatus()),
			attribute.String("agent.error", e.AgentEnd.GetErrorMessage()),
		)
	case *agent_observability.AgentEvent_CustomEvent:
		span.SetName(fmt.Sprintf("Custom Event: %s", e.CustomEvent.GetEventName()))
		span.SetAttributes(
			attribute.String("custom.event_name", e.CustomEvent.GetEventName()),
			attribute.StringMap("custom.properties", e.CustomEvent.GetProperties()),
		)
	}

	log.Printf("Received event: %T for agent %s, session %s", e, event.GetAgentId(), event.GetSessionId())
	return &agent_observability.SendEventResponse{Success: true, Message: "Event received"}, nil
}

// initTracer creates a new tracer provider with a stdout exporter.
func initTracer() (*trace.TracerProvider, error) {
	// Create stdout exporter to be able to see the collected traces.
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
	}

	// For a real application, you'd use an OTLP exporter to send to a collector:
	// otlpExporter, err := otlptracegrpc.New(context.Background())
	// if err != nil { ... }
	// tp := trace.NewTracerProvider(
	// 	trace.WithBatcher(otlpExporter),
	// 	trace.WithResource(resource.NewWithAttributes(
	// 		semconv.SchemaURL,
	// 		semconv.ServiceName("agent-observability-service"),
	// 	)),
	// )

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("agent-observability-service"),
			attribute.String("environment", "development"),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

func main() {
	tp, err := initTracer()
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	tracer := otel.Tracer("agent-observability-service")
	agent_observability.RegisterAgentObservabilityServiceServer(s, NewAgentObservabilityServer(tracer))
	log.Printf("server listening at %v", lis.Addr())

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		for range c {
			// sig is a ^C, handle it
			log.Println("shutting down gRPC server...")
			s.GracefulStop()
			<-time.After(500 * time.Millisecond) // Give some time for shutdown
			os.Exit(0)
		}
	}()

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
