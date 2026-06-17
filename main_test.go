package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestTwoPlayerRound(t *testing.T) {
	resetServerState()
	server := httptest.NewServer(httpHandler())
	defer server.Close()

	first := connectTestPlayer(t, server.URL)
	defer first.Close()
	second := connectTestPlayer(t, server.URL)
	defer second.Close()

	writeTestMessage(t, first, clientMessage{Type: "join_queue"})
	writeTestMessage(t, second, clientMessage{Type: "join_queue"})

	readUntilType(t, first, "waiting")
	readUntilType(t, second, "waiting")
	readUntilType(t, first, "game_started")
	readUntilType(t, second, "game_started")

	writeTestMessage(t, first, clientMessage{Type: "submit_move", Move: "rock"})
	writeTestMessage(t, second, clientMessage{Type: "submit_move", Move: "scissors"})

	firstResult := readUntilType(t, first, "round_result")
	secondResult := readUntilType(t, second, "round_result")

	if firstResult.Result != "win" {
		t.Fatalf("first player result = %q, want win", firstResult.Result)
	}
	if secondResult.Result != "lose" {
		t.Fatalf("second player result = %q, want lose", secondResult.Result)
	}
}

func TestOpponentDisconnectRequeuesRemainingPlayer(t *testing.T) {
	resetServerState()
	server := httptest.NewServer(httpHandler())
	defer server.Close()

	first := connectTestPlayer(t, server.URL)
	second := connectTestPlayer(t, server.URL)
	defer second.Close()

	writeTestMessage(t, first, clientMessage{Type: "join_queue"})
	writeTestMessage(t, second, clientMessage{Type: "join_queue"})

	readUntilType(t, first, "game_started")
	readUntilType(t, second, "game_started")

	if err := first.Close(); err != nil {
		t.Fatalf("close first player: %v", err)
	}

	readUntilType(t, second, "opponent_left")
	readUntilType(t, second, "waiting")
}

func httpHandler() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRequest)
	return mux
}

func resetServerState() {
	stateMu.Lock()
	defer stateMu.Unlock()

	players = map[string]*player{}
	games = map[string]*game{}
	waitingPlayers = nil
}

func connectTestPlayer(t *testing.T, serverURL string) *websocket.Conn {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(serverURL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("connect test player: %v", err)
	}

	return conn
}

func writeTestMessage(t *testing.T, conn *websocket.Conn, message clientMessage) {
	t.Helper()

	if err := conn.WriteJSON(message); err != nil {
		t.Fatalf("write message: %v", err)
	}
}

func readUntilType(t *testing.T, conn *websocket.Conn, messageType string) serverMessage {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}

		var message serverMessage
		if err := conn.ReadJSON(&message); err != nil {
			continue
		}

		if message.Type == messageType {
			return message
		}
	}

	t.Fatalf("timed out waiting for message type %q", messageType)
	return serverMessage{}
}
