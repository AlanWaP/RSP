const statusEl = document.querySelector("#status");
const serverUrlInput = document.querySelector("#server-url");
const connectButton = document.querySelector("#connect-button");
const connectionPanel = document.querySelector("#connection-panel");
const playerLabel = document.querySelector("#player-label");
const gameLabel = document.querySelector("#game-label");
const resultTitle = document.querySelector("#result-title");
const resultDetail = document.querySelector("#result-detail");
const playAgainButton = document.querySelector("#play-again-button");
const leaveButton = document.querySelector("#leave-button");
const choiceButtons = Array.from(document.querySelectorAll(".choice-button"));

const urlParams = new URLSearchParams(window.location.search);
const savedServerUrl = localStorage.getItem("rpsServerUrl");
const defaultServerUrl =
  urlParams.get("server") ||
  savedServerUrl ||
  (window.location.hostname === "localhost" ||
  window.location.hostname === "127.0.0.1"
    ? "ws://localhost:3000"
    : "");

let socket;
let playerId;
let gameId;
let submittedMove;

serverUrlInput.value = defaultServerUrl;
setChoicesEnabled(false);

if (defaultServerUrl) {
  connect(defaultServerUrl);
}

connectButton.addEventListener("click", () => {
  connect(serverUrlInput.value.trim());
});

serverUrlInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    connect(serverUrlInput.value.trim());
  }
});

choiceButtons.forEach((button) => {
  button.addEventListener("click", () => {
    const move = button.dataset.move;

    if (!socket || socket.readyState !== WebSocket.OPEN || submittedMove) {
      return;
    }

    submittedMove = move;
    button.classList.add("selected");
    setChoicesEnabled(false);
    send({ type: "submit_move", move });
    setStatus("Move submitted. Waiting for your opponent...");
    setResult("Move locked", "Your choice will be revealed after both players submit.");
  });
});

playAgainButton.addEventListener("click", () => {
  resetRound();
  send({ type: "play_again" });
  setStatus("Waiting for the next round...");
  setResult("Ready for another round", "Waiting for your opponent to be ready.");
  playAgainButton.hidden = true;
});

leaveButton.addEventListener("click", () => {
  resetRound();
  send({ type: "leave_game" });
  setStatus("Looking for a new opponent...");
  setResult("Back in queue", "You will be matched when another player is waiting.");
  playAgainButton.hidden = true;
});

function connect(rawUrl) {
  if (!rawUrl) {
    setStatus("Enter your backend WebSocket URL to start.");
    return;
  }

  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.close();
  }

  setStatus("Connecting to game server...");
  connectButton.disabled = true;
  localStorage.setItem("rpsServerUrl", rawUrl);

  socket = new WebSocket(rawUrl);

  socket.addEventListener("open", () => {
    connectionPanel.hidden = true;
    connectButton.disabled = false;
    setStatus("Connected. Waiting for another player...");
    send({ type: "join_queue" });
  });

  socket.addEventListener("message", (event) => {
    handleServerMessage(event.data);
  });

  socket.addEventListener("close", () => {
    connectionPanel.hidden = false;
    connectButton.disabled = false;
    setChoicesEnabled(false);
    playAgainButton.hidden = true;
    leaveButton.hidden = true;
    playerId = undefined;
    gameId = undefined;
    updateLabels();
    setStatus("Disconnected from game server.");
    setResult("Connection closed", "Reconnect when your backend server is available.");
  });

  socket.addEventListener("error", () => {
    setStatus("Could not connect. Check the backend URL and server status.");
    connectButton.disabled = false;
  });
}

function handleServerMessage(rawMessage) {
  let message;

  try {
    message = JSON.parse(rawMessage);
  } catch {
    return;
  }

  if (message.type === "waiting") {
    playerId = message.playerId || playerId;
    gameId = undefined;
    resetRound();
    updateLabels();
    setStatus("Waiting for another player...");
    setResult("Waiting room", "Keep this page open while the backend finds a match.");
    leaveButton.hidden = true;
    return;
  }

  if (message.type === "game_started") {
    playerId = message.playerId;
    gameId = message.gameId;
    resetRound();
    updateLabels();
    setChoicesEnabled(true);
    setStatus("Game started. Choose rock, paper, or scissors.");
    setResult("Choose your move", "Your opponent will not see it until both moves are in.");
    playAgainButton.hidden = true;
    leaveButton.hidden = false;
    return;
  }

  if (message.type === "opponent_moved") {
    if (submittedMove) {
      setStatus("Both moves are almost ready...");
    } else {
      setStatus("Your opponent has moved. Choose your move.");
    }
    return;
  }

  if (message.type === "round_result") {
    setChoicesEnabled(false);
    showRoundResult(message);
    playAgainButton.hidden = false;
    leaveButton.hidden = false;
    return;
  }

  if (message.type === "opponent_left") {
    resetRound();
    gameId = undefined;
    updateLabels();
    setChoicesEnabled(false);
    setStatus("Your opponent left. Waiting for a new player...");
    setResult("Opponent left", "The backend has returned you to the waiting queue.");
    playAgainButton.hidden = true;
    leaveButton.hidden = true;
    return;
  }

  if (message.type === "error") {
    setStatus(message.message || "The server reported an error.");
  }
}

function showRoundResult(message) {
  const resultText = {
    win: "You won",
    lose: "You lost",
    draw: "Draw",
  };

  setStatus("Round complete.");
  setResult(
    resultText[message.result] || "Round complete",
    `You chose ${formatMove(message.yourMove)}. Your opponent chose ${formatMove(
      message.opponentMove
    )}.`
  );
}

function send(message) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify(message));
  }
}

function setStatus(text) {
  statusEl.textContent = text;
}

function setResult(title, detail) {
  resultTitle.textContent = title;
  resultDetail.textContent = detail;
}

function setChoicesEnabled(enabled) {
  choiceButtons.forEach((button) => {
    button.disabled = !enabled;
  });
}

function resetRound() {
  submittedMove = undefined;
  choiceButtons.forEach((button) => button.classList.remove("selected"));
  setChoicesEnabled(false);
}

function updateLabels() {
  playerLabel.textContent = `Player: ${playerId || "not assigned"}`;
  gameLabel.textContent = `Game: ${gameId || "none"}`;
}

function formatMove(move) {
  if (!move) {
    return "nothing";
  }

  return move[0].toUpperCase() + move.slice(1);
}
