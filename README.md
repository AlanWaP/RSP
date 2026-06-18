# Multiplayer Rock Paper Scissors

A small two-player Rock Paper Scissors game. The browser UI is a static site
that can be hosted on GitHub Pages, while the realtime matchmaking backend runs
as a lightweight Go WebSocket server on your PC or another host.

## How It Works

GitHub Pages only serves static files. It hosts:

- `index.html`
- `style.css`
- `script.js`

The backend runs separately from GitHub Pages:

- `main.go` starts a Go WebSocket server.
- Players connect from the page to the backend, then explicitly enter the
  waiting queue.
- The backend puts players into a waiting queue.
- When two players are waiting, the backend creates a game room.
- Each browser submits one move.
- The backend calculates the result and sends it to both browsers.
- After a round, each player can join a new game or return to the main page.

The browser page can connect to any backend URL you provide. During local
testing that URL is usually `ws://localhost:3000`. From GitHub Pages it must be
a secure `wss://` URL. The page also accepts a `server` query parameter and
stores the most recent backend URL in local storage.

## Run The Backend Locally

Install Go, then download dependencies:

```sh
go mod tidy
```

Start the backend and Cloudflare Tunnel on your PC:

```sh
./scripts/start-backend.sh
```

The script starts the Go backend, starts `cloudflared`, waits for the public
tunnel URL, then prints the full GitHub Pages URL to open. It will look like:

```text
https://AlanWaP.github.io/RSP/?server=wss://example.trycloudflare.com
```

By default the local backend listens here:

```text
ws://localhost:3000
```

You can change the port with the `PORT` environment variable:

```sh
PORT=4000 ./scripts/start-backend.sh
```

Or pass the port as the first argument:

```sh
./scripts/start-backend.sh 4000
```

You need `cloudflared` installed:

```sh
brew install cloudflare/cloudflare/cloudflared
```

To build a reusable backend executable:

```sh
go build -o rsp-server .
./rsp-server
```

## Test Locally

Start the backend first, then open `index.html` directly in two browser tabs.
If that is blocked by browser file restrictions, serve the directory with a
simple static server:

```sh
python3 -m http.server 8080
```

Then open:

```text
http://localhost:8080
```

The page will automatically use `ws://localhost:3000` when opened from
`localhost`. Open it in two tabs or two browsers, connect both tabs, then click
`Enter waiting queue` in each tab to simulate two players.

You can also run the backend tests:

```sh
go test ./...
```

## Expose Your PC Backend

When the frontend is on GitHub Pages, other players need a public HTTPS/WSS URL
that reaches the backend on your PC. The startup script runs Cloudflare Tunnel
for you:

```sh
./scripts/start-backend.sh
```

The tunnel tool prints a public HTTPS URL. The script converts that URL to a
WebSocket URL by using `wss://` and prints the complete game URL. For example:

```text
https://example-tunnel.trycloudflare.com
```

becomes:

```text
wss://example-tunnel.trycloudflare.com
```

Keep your PC awake and keep `./scripts/start-backend.sh` running while people
are playing. If the backend or tunnel stops, players will disconnect.

## Deploy The Frontend To GitHub Pages

The frontend is already hosted by GitHub Pages. To play from that page, start
the backend and Cloudflare Tunnel:

```sh
./scripts/start-backend.sh
```

The script prints the complete frontend URL to open in your browser:

```text
https://AlanWaP.github.io/RSP/?server=wss://example.trycloudflare.com
```

Open that URL in two browser tabs or share it with another player while the
script keeps running. The `server` query parameter tells the static frontend
which live backend tunnel to use.

## Important Notes

- GitHub Pages cannot run the backend. It only hosts the static browser files.
- The backend currently stores games in memory, so all games reset when the
  server restarts.
- Leaving a game does not automatically place either player back in matchmaking.
  Players choose whether to enter the queue again.
- Do not expose this as a serious public service from your PC without thinking
  about security, rate limiting, and uptime.
- For casual testing with friends, a local backend plus Cloudflare Tunnel or
  ngrok is enough.
