package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"agent-observability-prototype/agent_observability"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	address = "localhost:50051"
)

func main() {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := agent_observability.NewAgentObservabilityServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	agentID := uuid.New().String()
	sessionID := uuid.New().String()

	log.Printf("Simulating agent %s in session %s...", agentID, sessionID)

	// Simulate Agent Start
	sendEvent(ctx, c, &agent_observability.AgentEvent{
		EventId:   uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		AgentId:   agentID,
		SessionId: sessionID,
		EventType: &agent_observability.AgentEvent_AgentStart{
			AgentStart: &agent_observability.AgentStartEvent{
				InitialGoal: "Plan a trip to Paris for 3 days",
				Context: map[string]string{
					"user_id":  "user-123",
					"priority": "high",
				},
			},
		},
	})
	time.Sleep(50 * time.Millisecond)

	// Simulate Agent Decision to use a tool
	sendEvent(ctx, c, &agent_observability.AgentEvent{
		EventId:   uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		AgentId:   agentID,
		SessionId: sessionID,
		EventType: &agent_observability.AgentEvent_AgentDecision{
			AgentDecision: &agent_observability.AgentDecisionEvent{
				Thought:     "User wants a trip to Paris. I should first find flights.",
				DecisionType: "tool_call",
				Parameters: map[string]string{
					"tool_name": "FlightSearch",
					"destination": "Paris",
					"duration": "3 days",
				},
			},
		},
	})
	time.Sleep(100 * time.Millisecond)

	// Simulate Tool Call
	flightCallID := uuid.New().String()
	sendEvent(ctx, c, &agent_observability.AgentEvent{
		EventId:   uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		AgentId:   agentID,
		SessionId: sessionID,
		EventType: &agent_observability.AgentEvent_ToolCall{
			ToolCall: &agent_observability.ToolCallEvent{
				ToolName: "FlightSearch",
				ToolInput: fmt.Sprintf(`{"destination": "Paris", "start_date": "%s", "end_date": "%s"}`,
					time.Now().AddDate(0, 1, 0).Format("2006-01-02"), time.Now().AddDate(0, 1, 3).Format("2006-01-02")),
				CallId: flightCallID,
			},
		},
	})
	time.Sleep(150 * time.Millisecond)

	// Simulate LLM Interaction (e.g., parsing tool output or generating response)
	sendEvent(ctx, c, &agent_observability.AgentEvent{
		EventId:   uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		AgentId:   agentID,
		SessionId: sessionID,
		EventType: &agent_observability.AgentEvent_LlmInteraction{
			LlmInteraction: &agent_observability.LLMInteractionEvent{
				ModelName: "gemini-pro",
				Prompt:    "User asked for flights to Paris. Here are the results from FlightSearch: ... What should I do next?",
				Response:  "The user will likely want hotel recommendations now. I should call the HotelSearch tool.",
				Metadata: map[string]string{
					"temperature": "0.7",
					"max_tokens":  "100",
				},
			},
		},
	})
	time.Sleep(100 * time.Millisecond)

	// Simulate Tool Result (success)
	sendEvent(ctx, c, &agent_observability.AgentEvent{
		EventId:   uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		AgentId:   agentID,
		SessionId: sessionID,
		EventType: &agent_observability.AgentEvent_ToolResult{
			ToolResult: &agent_observability.ToolResultEvent{
				ToolName:    "FlightSearch",
				ToolOutput:  `{"flights": [{"id": "F1", "price": "€150"}, {"id": "F2", "price": "€180"}]}`,
				CallId:      flightCallID,
				Success:     true,
			},
		},
	})
	time.Sleep(50 * time.Millisecond)

	// Simulate Agent Decision to search for hotels
	sendEvent(ctx, c, &agent_observability.AgentEvent{
		EventId:   uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		AgentId:   agentID,
		SessionId: sessionID,
		EventType: &agent_observability.AgentEvent_AgentDecision{
			AgentDecision: &agent_observability.AgentDecisionEvent{
				Thought:     "Flights found. Now I need to find hotels in Paris.",
				DecisionType: "tool_call",
				Parameters: map[string]string{
					"tool_name": "HotelSearch",
					"location": "Paris",
				},
			},
		},
	})
	time.Sleep(100 * time.Millisecond)

	// Simulate Tool Call (Hotel Search)
	hotelCallID := uuid.New().String()
	sendEvent(ctx, c, &agent_observability.AgentEvent{
		EventId:   uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		AgentId:   agentID,
		SessionId: sessionID,
		EventType: &agent_observability.AgentEvent_ToolCall{
			ToolCall: &agent_observability.ToolCallEvent{
				ToolName: "HotelSearch",
				ToolInput: `{"location": "Paris", "min_stars": 4}`,
				CallId: hotelCallID,
			},
		},
	})
	time.Sleep(150 * time.Millisecond)

	// Simulate Tool Result (failure)
	sendEvent(ctx, c, &agent_observability.AgentEvent{
		EventId:   uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		AgentId:   agentID,
		SessionId: sessionID,
		EventType: &agent_observability.AgentEvent_ToolResult{
			ToolResult: &agent_observability.ToolResultEvent{
				ToolName:    "HotelSearch",
				ToolOutput:  "",
				CallId:      hotelCallID,
				Success:     false,
				ErrorMessage: "API rate limit exceeded for HotelSearch.",
			},
		},
	})
	time.Sleep(50 * time.Millisecond)

	// Simulate Agent Decision to inform user about failure
	sendEvent(ctx, c, &agent_observability.AgentEvent{
		EventId:   uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		AgentId:   agentID,
		SessionId: sessionID,
		EventType: &agent_observability.AgentEvent_AgentDecision{
			AgentDecision: &agent_observability.AgentDecisionEvent{
				Thought:     "Hotel search failed. I should inform the user and suggest retrying or checking manually.",
				DecisionType: "respond",
				Parameters: map[string]string{
					"response": "I apologize, but I encountered an issue searching for hotels. Would you like me to try again?",
				},
			},
		},
	})
	time.Sleep(100 * time.Millisecond)

	// Simulate a custom internal event
	sendEvent(ctx, c, &agent_observability.AgentEvent{
		EventId:   uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		AgentId:   agentID,
		SessionId: sessionID,
		EventType: &agent_observability.AgentEvent_CustomEvent{
			CustomEvent: &agent_observability.CustomEvent{
				EventName: "MemoryUpdate",
				Properties: map[string]string{
					"entry_count": "5",
					"summary_length": "200",
				},
			},
		},
	})
	time.Sleep(50 * time.Millisecond)

	// Simulate Agent End
	sendEvent(ctx, c, &agent_observability.AgentEvent{
		EventId:   uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		AgentId:   agentID,
		SessionId: sessionID,
		EventType: &agent_observability.AgentEvent_AgentEnd{
			AgentEnd: &agent_observability.AgentEndEvent{
				FinalResult: "Provided flight options and informed about hotel search issue.",
				Status:      "completed_with_issues",
			},
		},
	})
	log.Println("Agent simulation complete.")
}

func sendEvent(ctx context.Context, c agent_observability.AgentObservabilityServiceClient, event *agent_observability.AgentEvent) {
	r, err := c.SendEvent(ctx, event)
	if err != nil {
		log.Printf("could not send event: %v", err)
	}
	if !r.GetSuccess() {
		log.Printf("event failed on server: %s", r.GetMessage())
	}
}
