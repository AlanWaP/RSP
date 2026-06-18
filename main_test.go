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

func TestOpponentDisconnectStopsGameForRemainingPlayer(t *testing.T) {
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
	assertNoMessageType(t, second, "waiting", "game_started")
}

func TestLeaveQueueKeepsPlayerOutOfMatchmaking(t *testing.T) {
	resetServerState()
	server := httptest.NewServer(httpHandler())
	defer server.Close()

	first := connectTestPlayer(t, server.URL)
	defer first.Close()
	second := connectTestPlayer(t, server.URL)
	defer second.Close()
	third := connectTestPlayer(t, server.URL)
	defer third.Close()

	writeTestMessage(t, first, clientMessage{Type: "join_queue"})
	readUntilType(t, first, "waiting")
	writeTestMessage(t, first, clientMessage{Type: "leave_game"})
	readUntilType(t, first, "left_game")

	writeTestMessage(t, second, clientMessage{Type: "join_queue"})
	writeTestMessage(t, third, clientMessage{Type: "join_queue"})

	readUntilType(t, second, "game_started")
	readUntilType(t, third, "game_started")
	assertNoMessageType(t, first, "waiting", "game_started")
}

func TestLeaveGameDoesNotRequeueLeavingPlayer(t *testing.T) {
	resetServerState()
	server := httptest.NewServer(httpHandler())
	defer server.Close()

	first := connectTestPlayer(t, server.URL)
	defer first.Close()
	second := connectTestPlayer(t, server.URL)
	defer second.Close()
	third := connectTestPlayer(t, server.URL)
	defer third.Close()

	writeTestMessage(t, first, clientMessage{Type: "join_queue"})
	writeTestMessage(t, second, clientMessage{Type: "join_queue"})

	readUntilType(t, first, "game_started")
	readUntilType(t, second, "game_started")

	writeTestMessage(t, third, clientMessage{Type: "join_queue"})
	readUntilType(t, third, "waiting")

	writeTestMessage(t, first, clientMessage{Type: "leave_game"})

	readUntilType(t, second, "opponent_left")
	assertNoMessageType(t, second, "waiting", "game_started")
	assertNoMessageType(t, third, "game_started")
	assertNoMessageType(t, first, "waiting", "game_started")
}

func TestFinishedPlayerCanJoinNewQueueWithoutStoppingOpponent(t *testing.T) {
	resetServerState()
	server := httptest.NewServer(httpHandler())
	defer server.Close()

	first := connectTestPlayer(t, server.URL)
	defer first.Close()
	second := connectTestPlayer(t, server.URL)
	defer second.Close()
	third := connectTestPlayer(t, server.URL)
	defer third.Close()

	writeTestMessage(t, first, clientMessage{Type: "join_queue"})
	writeTestMessage(t, second, clientMessage{Type: "join_queue"})
	readUntilType(t, first, "game_started")
	readUntilType(t, second, "game_started")

	writeTestMessage(t, first, clientMessage{Type: "submit_move", Move: "rock"})
	writeTestMessage(t, second, clientMessage{Type: "submit_move", Move: "scissors"})
	readUntilType(t, first, "round_result")
	readUntilType(t, second, "round_result")

	writeTestMessage(t, first, clientMessage{Type: "join_queue"})
	readUntilType(t, first, "waiting")
	writeTestMessage(t, third, clientMessage{Type: "join_queue"})

	readUntilType(t, first, "game_started")
	readUntilType(t, third, "game_started")
	assertNoMessageType(t, second, "opponent_left", "game_started")
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

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	for {
		var message serverMessage
		if err := conn.ReadJSON(&message); err != nil {
			t.Fatalf("read message while waiting for type %q: %v", messageType, err)
		}

		if message.Type == messageType {
			return message
		}
	}
}

func assertNoMessageType(t *testing.T, conn *websocket.Conn, forbiddenTypes ...string) {
	t.Helper()

	forbidden := map[string]bool{}
	for _, messageType := range forbiddenTypes {
		forbidden[messageType] = true
	}

	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	var message serverMessage
	if err := conn.ReadJSON(&message); err != nil {
		return
	}

	if forbidden[message.Type] {
		t.Fatalf("received forbidden message type %q", message.Type)
	}
}
