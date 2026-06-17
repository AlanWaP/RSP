const http = require("http");
const crypto = require("crypto");
const { WebSocket, WebSocketServer } = require("ws");

const PORT = Number(process.env.PORT || 3000);
const MOVES = new Set(["rock", "paper", "scissors"]);

const players = new Map();
const games = new Map();
let waitingPlayers = [];

const server = http.createServer((req, res) => {
  res.writeHead(200, { "content-type": "text/plain; charset=utf-8" });
  res.end("Rock Paper Scissors WebSocket server is running.\n");
});

const wss = new WebSocketServer({ server });

wss.on("connection", (socket) => {
  const player = {
    id: createId("player"),
    socket,
    gameId: undefined,
    readyForRematch: false,
  };

  players.set(player.id, player);

  socket.on("message", (rawMessage) => {
    handleMessage(player, rawMessage);
  });

  socket.on("close", () => {
    removePlayer(player);
  });
});

server.listen(PORT, () => {
  console.log(`RPS WebSocket server listening on ws://localhost:${PORT}`);
});

function handleMessage(player, rawMessage) {
  let message;

  try {
    message = JSON.parse(rawMessage);
  } catch {
    sendError(player, "Message must be valid JSON.");
    return;
  }

  if (message.type === "join_queue") {
    joinQueue(player);
    return;
  }

  if (message.type === "submit_move") {
    submitMove(player, message.move);
    return;
  }

  if (message.type === "play_again") {
    markReadyForRematch(player);
    return;
  }

  if (message.type === "leave_game") {
    leaveGame(player, { requeue: true });
    return;
  }

  sendError(player, `Unknown message type: ${message.type}`);
}

function joinQueue(player) {
  if (!isOpen(player.socket)) {
    return;
  }

  if (player.gameId) {
    leaveGame(player, { requeue: false });
  }

  player.readyForRematch = false;

  if (!waitingPlayers.includes(player.id)) {
    waitingPlayers.push(player.id);
  }

  send(player, { type: "waiting", playerId: player.id });
  matchWaitingPlayers();
}

function matchWaitingPlayers() {
  waitingPlayers = waitingPlayers.filter((playerId) => {
    const player = players.get(playerId);
    return player && isOpen(player.socket) && !player.gameId;
  });

  while (waitingPlayers.length >= 2) {
    const firstPlayer = players.get(waitingPlayers.shift());
    const secondPlayer = players.get(waitingPlayers.shift());

    if (!firstPlayer || !secondPlayer) {
      continue;
    }

    createGame(firstPlayer, secondPlayer);
  }
}

function createGame(firstPlayer, secondPlayer) {
  const gameId = createId("game");
  const game = {
    id: gameId,
    playerIds: [firstPlayer.id, secondPlayer.id],
    moves: new Map(),
    status: "playing",
  };

  games.set(gameId, game);
  firstPlayer.gameId = gameId;
  secondPlayer.gameId = gameId;
  firstPlayer.readyForRematch = false;
  secondPlayer.readyForRematch = false;

  for (const player of [firstPlayer, secondPlayer]) {
    send(player, {
      type: "game_started",
      gameId,
      playerId: player.id,
    });
  }
}

function submitMove(player, move) {
  if (!player.gameId) {
    sendError(player, "You are not in a game.");
    return;
  }

  if (!MOVES.has(move)) {
    sendError(player, "Move must be rock, paper, or scissors.");
    return;
  }

  const game = games.get(player.gameId);

  if (!game || game.status !== "playing") {
    sendError(player, "This game is not accepting moves.");
    return;
  }

  if (game.moves.has(player.id)) {
    sendError(player, "You already submitted a move for this round.");
    return;
  }

  game.moves.set(player.id, move);
  notifyOpponent(player, { type: "opponent_moved" });

  if (game.moves.size === 2) {
    finishRound(game);
  }
}

function finishRound(game) {
  const [firstPlayerId, secondPlayerId] = game.playerIds;
  const firstMove = game.moves.get(firstPlayerId);
  const secondMove = game.moves.get(secondPlayerId);
  const firstPlayer = players.get(firstPlayerId);
  const secondPlayer = players.get(secondPlayerId);

  game.status = "finished";

  if (firstPlayer) {
    send(firstPlayer, {
      type: "round_result",
      yourMove: firstMove,
      opponentMove: secondMove,
      result: getResult(firstMove, secondMove),
    });
  }

  if (secondPlayer) {
    send(secondPlayer, {
      type: "round_result",
      yourMove: secondMove,
      opponentMove: firstMove,
      result: getResult(secondMove, firstMove),
    });
  }
}

function markReadyForRematch(player) {
  const game = games.get(player.gameId);

  if (!game || game.status !== "finished") {
    sendError(player, "No finished game is ready for a rematch.");
    return;
  }

  player.readyForRematch = true;
  const gamePlayers = game.playerIds.map((playerId) => players.get(playerId));

  if (gamePlayers.every((gamePlayer) => gamePlayer && gamePlayer.readyForRematch)) {
    game.moves.clear();
    game.status = "playing";

    for (const gamePlayer of gamePlayers) {
      gamePlayer.readyForRematch = false;
      send(gamePlayer, {
        type: "game_started",
        gameId: game.id,
        playerId: gamePlayer.id,
      });
    }
  }
}

function leaveGame(player, { requeue }) {
  const game = games.get(player.gameId);

  if (!game) {
    removeFromQueue(player.id);

    if (requeue) {
      joinQueue(player);
    }

    return;
  }

  const opponent = getOpponent(player, game);
  games.delete(game.id);
  player.gameId = undefined;
  player.readyForRematch = false;
  removeFromQueue(player.id);

  if (opponent) {
    opponent.gameId = undefined;
    opponent.readyForRematch = false;
    send(opponent, { type: "opponent_left" });

    if (isOpen(opponent.socket)) {
      joinQueue(opponent);
    }
  }

  if (requeue && isOpen(player.socket)) {
    joinQueue(player);
  }
}

function removePlayer(player) {
  removeFromQueue(player.id);
  leaveGame(player, { requeue: false });
  players.delete(player.id);
}

function removeFromQueue(playerId) {
  waitingPlayers = waitingPlayers.filter((waitingPlayerId) => waitingPlayerId !== playerId);
}

function getOpponent(player, game = games.get(player.gameId)) {
  if (!game) {
    return undefined;
  }

  const opponentId = game.playerIds.find((playerId) => playerId !== player.id);
  return players.get(opponentId);
}

function notifyOpponent(player, message) {
  const opponent = getOpponent(player);

  if (opponent) {
    send(opponent, message);
  }
}

function getResult(yourMove, opponentMove) {
  if (yourMove === opponentMove) {
    return "draw";
  }

  if (
    (yourMove === "rock" && opponentMove === "scissors") ||
    (yourMove === "paper" && opponentMove === "rock") ||
    (yourMove === "scissors" && opponentMove === "paper")
  ) {
    return "win";
  }

  return "lose";
}

function send(player, message) {
  if (isOpen(player.socket)) {
    player.socket.send(JSON.stringify(message));
  }
}

function sendError(player, message) {
  send(player, { type: "error", message });
}

function isOpen(socket) {
  return socket.readyState === WebSocket.OPEN;
}

function createId(prefix) {
  return `${prefix}_${crypto.randomUUID().slice(0, 8)}`;
}
