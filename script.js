const statusEl = document.querySelector("#status");
const serverUrlInput = document.querySelector("#server-url");
const connectButton = document.querySelector("#connect-button");
const connectionPanel = document.querySelector("#connection-panel");
const playerLabel = document.querySelector("#player-label");
const gameLabel = document.querySelector("#game-label");
const resultTitle = document.querySelector("#result-title");
const resultDetail = document.querySelector("#result-detail");
const joinQueueButton = document.querySelector("#join-queue-button");
const playAgainButton = document.querySelector("#play-again-button");
const leaveButton = document.querySelector("#leave-button");
const mainButton = document.querySelector("#main-button");
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
let isQueued = false;

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

joinQueueButton.addEventListener("click", () => {
  joinQueue();
});

playAgainButton.addEventListener("click", () => {
  joinQueue();
});

leaveButton.addEventListener("click", () => {
  resetRound();
  send({ type: "leave_game" });
  isQueued = false;
  gameId = undefined;
  updateLabels();
  setStatus("You left the game.");
  setResult("Game stopped", "The other player was notified that you left.");
  showPostGameActions();
});

mainButton.addEventListener("click", () => {
  send({ type: "leave_game" });
  showMainPage();
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
    showMainPage();
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
    isQueued = false;
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
    isQueued = true;
    resetRound();
    updateLabels();
    setStatus("Waiting for another player...");
    setResult("Waiting room", "Keep this page open while the backend finds a match.");
    joinQueueButton.hidden = true;
    playAgainButton.hidden = true;
    leaveButton.hidden = true;
    mainButton.hidden = false;
    return;
  }

  if (message.type === "game_started") {
    playerId = message.playerId;
    gameId = message.gameId;
    isQueued = false;
    resetRound();
    updateLabels();
    setChoicesEnabled(true);
    setStatus("Game started. Choose rock, paper, or scissors.");
    setResult("Choose your move", "Your opponent will not see it until both moves are in.");
    joinQueueButton.hidden = true;
    playAgainButton.hidden = true;
    leaveButton.hidden = false;
    mainButton.hidden = true;
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
    isQueued = false;
    gameId = undefined;
    updateLabels();
    showRoundResult(message);
    showPostGameActions();
    return;
  }

  if (message.type === "opponent_left") {
    resetRound();
    isQueued = false;
    gameId = undefined;
    updateLabels();
    setChoicesEnabled(false);
    setStatus("The other player left.");
    setResult("Game stopped", "Choose whether to play a new game or return to the main page.");
    showPostGameActions();
    return;
  }

  if (message.type === "left_game") {
    isQueued = false;
    gameId = undefined;
    updateLabels();
    if (message.reason === "queue_left") {
      showMainPage();
    }
    return;
  }

  if (message.type === "not_queued") {
    isQueued = false;
    gameId = undefined;
    updateLabels();
    if (!mainButton.hidden) {
      showMainPage();
    }
    return;
  }

  if (message.type === "already_queued") {
    isQueued = true;
    gameId = undefined;
    updateLabels();
    setStatus("Already waiting for another player...");
    setResult("Waiting room", "Keep this page open while the backend finds a match.");
    joinQueueButton.hidden = true;
    playAgainButton.hidden = true;
    leaveButton.hidden = true;
    mainButton.hidden = false;
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

function joinQueue() {
  if (!socket || socket.readyState !== WebSocket.OPEN) {
    setStatus("Connect to the game server first.");
    return;
  }

  resetRound();
  gameId = undefined;
  isQueued = true;
  updateLabels();
  send({ type: "join_queue" });
  setStatus("Joining the waiting queue...");
  setResult("Joining queue", "Waiting for the backend to confirm your place.");
  joinQueueButton.hidden = true;
  playAgainButton.hidden = true;
  leaveButton.hidden = true;
  mainButton.hidden = false;
}

function showMainPage() {
  resetRound();
  gameId = undefined;
  isQueued = false;
  updateLabels();
  setStatus("Connected. Enter the queue when you want to play.");
  setResult("Main page", "You are not in the waiting queue yet.");
  joinQueueButton.hidden = false;
  playAgainButton.hidden = true;
  leaveButton.hidden = true;
  mainButton.hidden = true;
}

function showPostGameActions() {
  setChoicesEnabled(false);
  joinQueueButton.hidden = true;
  playAgainButton.hidden = false;
  leaveButton.hidden = true;
  mainButton.hidden = false;
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
