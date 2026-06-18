package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

const (
	statusPlaying  = "playing"
	statusFinished = "finished"
)

var validMoves = map[string]bool{
	"rock":     true,
	"paper":    true,
	"scissors": true,
}

type player struct {
	id              string
	conn            *websocket.Conn
	gameID          string
	readyForRematch bool
	writeMu         sync.Mutex
}

type game struct {
	id        string
	playerIDs [2]string
	moves     map[string]string
	status    string
}

type clientMessage struct {
	Type string `json:"type"`
	Move string `json:"move,omitempty"`
}

type serverMessage struct {
	Type         string `json:"type"`
	PlayerID     string `json:"playerId,omitempty"`
	GameID       string `json:"gameId,omitempty"`
	YourMove     string `json:"yourMove,omitempty"`
	OpponentMove string `json:"opponentMove,omitempty"`
	Result       string `json:"result,omitempty"`
	Message      string `json:"message,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

type outboundMessage struct {
	player *player
	body   serverMessage
}

var (
	stateMu        sync.Mutex
	players        = map[string]*player{}
	games          = map[string]*game{}
	waitingPlayers []string
)

var upgrader = websocket.Upgrader{
	// GitHub Pages and local tunnels are different origins from the backend.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	http.HandleFunc("/", handleRequest)

	log.Printf("RPS WebSocket server listening on ws://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		handleWebSocket(w, r)
		return
	}

	w.Header().Set("content-type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "Rock Paper Scissors WebSocket server is running.")
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	currentPlayer := &player{
		id:   createID("player"),
		conn: conn,
	}

	stateMu.Lock()
	players[currentPlayer.id] = currentPlayer
	stateMu.Unlock()

	defer removePlayer(currentPlayer)

	for {
		var message clientMessage
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		handleMessage(currentPlayer, message)
	}
}

func handleMessage(currentPlayer *player, message clientMessage) {
	switch message.Type {
	case "join_queue":
		joinQueue(currentPlayer)
	case "submit_move":
		submitMove(currentPlayer, message.Move)
	case "play_again":
		markReadyForRematch(currentPlayer)
	case "leave_game":
		leaveGame(currentPlayer, false)
	default:
		flushMessages([]outboundMessage{{
			player: currentPlayer,
			body: serverMessage{
				Type:    "error",
				Message: "Unknown message type: " + message.Type,
			},
		}})
	}
}

func joinQueue(currentPlayer *player) {
	stateMu.Lock()
	var messages []outboundMessage

	if currentPlayer.gameID != "" {
		leaveGameLocked(currentPlayer, false, &messages)
	}

	currentPlayer.readyForRematch = false
	enqueuePlayerLocked(currentPlayer, &messages)
	matchWaitingPlayersLocked(&messages)

	stateMu.Unlock()
	flushMessages(messages)
}

func enqueuePlayerLocked(currentPlayer *player, messages *[]outboundMessage) {
	if _, ok := players[currentPlayer.id]; !ok {
		return
	}

	for _, playerID := range waitingPlayers {
		if playerID == currentPlayer.id {
			*messages = append(*messages, outboundMessage{
				player: currentPlayer,
				body: serverMessage{
					Type:     "already_queued",
					PlayerID: currentPlayer.id,
				},
			})
			return
		}
	}

	waitingPlayers = append(waitingPlayers, currentPlayer.id)
	*messages = append(*messages, outboundMessage{
		player: currentPlayer,
		body: serverMessage{
			Type:     "waiting",
			PlayerID: currentPlayer.id,
		},
	})
}

func matchWaitingPlayersLocked(messages *[]outboundMessage) {
	waitingPlayers = compactWaitingPlayersLocked()

	for len(waitingPlayers) >= 2 {
		first := players[waitingPlayers[0]]
		second := players[waitingPlayers[1]]
		waitingPlayers = waitingPlayers[2:]

		if first == nil || second == nil {
			continue
		}

		createGameLocked(first, second, messages)
	}
}

func compactWaitingPlayersLocked() []string {
	active := make([]string, 0, len(waitingPlayers))

	for _, playerID := range waitingPlayers {
		currentPlayer := players[playerID]
		if currentPlayer != nil && currentPlayer.gameID == "" {
			active = append(active, playerID)
		}
	}

	return active
}

func createGameLocked(first *player, second *player, messages *[]outboundMessage) {
	gameID := createID("game")
	currentGame := &game{
		id:        gameID,
		playerIDs: [2]string{first.id, second.id},
		moves:     map[string]string{},
		status:    statusPlaying,
	}

	games[gameID] = currentGame
	first.gameID = gameID
	second.gameID = gameID
	first.readyForRematch = false
	second.readyForRematch = false

	for _, currentPlayer := range []*player{first, second} {
		*messages = append(*messages, outboundMessage{
			player: currentPlayer,
			body: serverMessage{
				Type:     "game_started",
				GameID:   gameID,
				PlayerID: currentPlayer.id,
			},
		})
	}
}

func submitMove(currentPlayer *player, move string) {
	stateMu.Lock()
	var messages []outboundMessage

	if !validMoves[move] {
		messages = append(messages, errorMessage(currentPlayer, "Move must be rock, paper, or scissors."))
		stateMu.Unlock()
		flushMessages(messages)
		return
	}

	currentGame := games[currentPlayer.gameID]
	if currentGame == nil {
		messages = append(messages, errorMessage(currentPlayer, "You are not in a game."))
		stateMu.Unlock()
		flushMessages(messages)
		return
	}

	if currentGame.status != statusPlaying {
		messages = append(messages, errorMessage(currentPlayer, "This game is not accepting moves."))
		stateMu.Unlock()
		flushMessages(messages)
		return
	}

	if _, exists := currentGame.moves[currentPlayer.id]; exists {
		messages = append(messages, errorMessage(currentPlayer, "You already submitted a move for this round."))
		stateMu.Unlock()
		flushMessages(messages)
		return
	}

	currentGame.moves[currentPlayer.id] = move

	if opponent := getOpponentLocked(currentPlayer, currentGame); opponent != nil {
		messages = append(messages, outboundMessage{
			player: opponent,
			body:   serverMessage{Type: "opponent_moved"},
		})
	}

	if len(currentGame.moves) == 2 {
		finishRoundLocked(currentGame, &messages)
	}

	stateMu.Unlock()
	flushMessages(messages)
}

func finishRoundLocked(currentGame *game, messages *[]outboundMessage) {
	firstID := currentGame.playerIDs[0]
	secondID := currentGame.playerIDs[1]
	firstMove := currentGame.moves[firstID]
	secondMove := currentGame.moves[secondID]
	currentGame.status = statusFinished

	if first := players[firstID]; first != nil {
		*messages = append(*messages, outboundMessage{
			player: first,
			body: serverMessage{
				Type:         "round_result",
				YourMove:     firstMove,
				OpponentMove: secondMove,
				Result:       getResult(firstMove, secondMove),
			},
		})
	}

	if second := players[secondID]; second != nil {
		*messages = append(*messages, outboundMessage{
			player: second,
			body: serverMessage{
				Type:         "round_result",
				YourMove:     secondMove,
				OpponentMove: firstMove,
				Result:       getResult(secondMove, firstMove),
			},
		})
	}
}

func markReadyForRematch(currentPlayer *player) {
	stateMu.Lock()
	var messages []outboundMessage

	currentGame := games[currentPlayer.gameID]
	if currentGame == nil || currentGame.status != statusFinished {
		messages = append(messages, errorMessage(currentPlayer, "No finished game is ready for a rematch."))
		stateMu.Unlock()
		flushMessages(messages)
		return
	}

	currentPlayer.readyForRematch = true
	first := players[currentGame.playerIDs[0]]
	second := players[currentGame.playerIDs[1]]

	if first != nil && second != nil && first.readyForRematch && second.readyForRematch {
		currentGame.moves = map[string]string{}
		currentGame.status = statusPlaying
		first.readyForRematch = false
		second.readyForRematch = false

		for _, playerInGame := range []*player{first, second} {
			messages = append(messages, outboundMessage{
				player: playerInGame,
				body: serverMessage{
					Type:     "game_started",
					GameID:   currentGame.id,
					PlayerID: playerInGame.id,
				},
			})
		}
	}

	stateMu.Unlock()
	flushMessages(messages)
}

func leaveGame(currentPlayer *player, requeue bool) {
	stateMu.Lock()
	var messages []outboundMessage
	leaveGameLocked(currentPlayer, requeue, &messages)
	stateMu.Unlock()
	flushMessages(messages)
}

func leaveGameLocked(currentPlayer *player, requeue bool, messages *[]outboundMessage) {
	wasQueued := removeFromQueueLocked(currentPlayer.id)

	currentGame := games[currentPlayer.gameID]
	if currentGame == nil {
		currentPlayer.gameID = ""
		currentPlayer.readyForRematch = false
		if requeue {
			enqueuePlayerLocked(currentPlayer, messages)
			matchWaitingPlayersLocked(messages)
			return
		}

		messageType := "not_queued"
		reason := ""
		if wasQueued {
			messageType = "left_game"
			reason = "queue_left"
		}
		*messages = append(*messages, outboundMessage{
			player: currentPlayer,
			body: serverMessage{
				Type:   messageType,
				Reason: reason,
			},
		})
		return
	}

	if currentGame.status == statusFinished {
		delete(games, currentGame.id)
		currentPlayer.gameID = ""
		currentPlayer.readyForRematch = false

		if requeue {
			enqueuePlayerLocked(currentPlayer, messages)
			matchWaitingPlayersLocked(messages)
		}

		return
	}

	delete(games, currentGame.id)
	currentPlayer.gameID = ""
	currentPlayer.readyForRematch = false

	if opponent := getOpponentLocked(currentPlayer, currentGame); opponent != nil {
		opponent.gameID = ""
		opponent.readyForRematch = false
		*messages = append(*messages, outboundMessage{
			player: opponent,
			body:   serverMessage{Type: "opponent_left"},
		})
	}

	if requeue {
		enqueuePlayerLocked(currentPlayer, messages)
	}

	matchWaitingPlayersLocked(messages)
}

func removePlayer(currentPlayer *player) {
	stateMu.Lock()
	var messages []outboundMessage
	removeFromQueueLocked(currentPlayer.id)
	leaveGameLocked(currentPlayer, false, &messages)
	delete(players, currentPlayer.id)
	stateMu.Unlock()

	flushMessages(messages)
	currentPlayer.conn.Close()
}

func removeFromQueueLocked(playerID string) bool {
	removed := false
	filtered := waitingPlayers[:0]
	for _, waitingPlayerID := range waitingPlayers {
		if waitingPlayerID != playerID {
			filtered = append(filtered, waitingPlayerID)
		} else {
			removed = true
		}
	}
	waitingPlayers = filtered
	return removed
}

func getOpponentLocked(currentPlayer *player, currentGame *game) *player {
	for _, playerID := range currentGame.playerIDs {
		if playerID != currentPlayer.id {
			return players[playerID]
		}
	}

	return nil
}

func errorMessage(currentPlayer *player, message string) outboundMessage {
	return outboundMessage{
		player: currentPlayer,
		body: serverMessage{
			Type:    "error",
			Message: message,
		},
	}
}

func flushMessages(messages []outboundMessage) {
	for _, message := range messages {
		send(message.player, message.body)
	}
}

func send(currentPlayer *player, message serverMessage) {
	if currentPlayer == nil {
		return
	}

	currentPlayer.writeMu.Lock()
	defer currentPlayer.writeMu.Unlock()

	if err := currentPlayer.conn.WriteJSON(message); err != nil {
		log.Printf("send to %s failed: %v", currentPlayer.id, err)
	}
}

func getResult(yourMove string, opponentMove string) string {
	if yourMove == opponentMove {
		return "draw"
	}

	if (yourMove == "rock" && opponentMove == "scissors") ||
		(yourMove == "paper" && opponentMove == "rock") ||
		(yourMove == "scissors" && opponentMove == "paper") {
		return "win"
	}

	return "lose"
}

func createID(prefix string) string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(bytes)
}

func decodeServerMessage(raw []byte) (serverMessage, error) {
	var message serverMessage
	err := json.Unmarshal(raw, &message)
	return message, err
}
