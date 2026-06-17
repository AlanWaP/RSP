# Multiplayer Rock Paper Scissors

This is the old README from when the project used a Node.js WebSocket backend.
It is kept for study and comparison only. The active project README is
`../README.md`, and the active backend is the Go server in `../main.go`.

## Original Node Backend Shape

A small two-player Rock Paper Scissors game. The browser UI is a static site
that can be hosted on GitHub Pages, while the realtime matchmaking backend runs
as a lightweight WebSocket server on your PC.

GitHub Pages only serves static files:

- `index.html`
- `style.css`
- `script.js`

The backend ran separately from GitHub Pages:

- `server.js` started a Node.js WebSocket server.
- Players connected from the page to the backend.
- The backend put players into a waiting queue.
- When two players were waiting, the backend created a game room.
- Each browser submitted one move.
- The backend calculated the result and sent it to both browsers.

## Run The Old Node Backend Locally

From the study-only Node backend directory:

```sh
cd study/node-backend
npm install
npm start
```

By default the backend listened here:

```text
ws://localhost:3000
```

The port could be changed with:

```sh
PORT=4000 npm start
```

## GitHub Pages Usage

When the frontend is on GitHub Pages, players need a public HTTPS/WSS URL that
reaches the backend. A tunnel such as Cloudflare Tunnel or ngrok can expose the
local backend.

Cloudflare Tunnel example:

```sh
cloudflared tunnel --url http://localhost:3000
```

ngrok example:

```sh
ngrok http 3000
```

The tunnel URL should be used as `wss://` from the GitHub Pages frontend:

```text
https://YOUR_USERNAME.github.io/YOUR_REPO/?server=wss://YOUR_TUNNEL_URL
```

## Notes

- GitHub Pages cannot run the backend.
- The Node backend stored games in memory.
- This README is historical. Use the root `README.md` for the current Go
  backend.
