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

## Prerequisites

Install these before running the game:

- Go, for running the backend.
- Python 3, optional but useful for serving the frontend locally.
- `cloudflared`, only needed when playing from GitHub Pages or sharing with
  another player over the internet.

On macOS, you can install `cloudflared` with Homebrew:

```sh
brew install cloudflare/cloudflare/cloudflared
```

After installing Go, download the Go dependencies once:

```sh
go mod tidy
```

## Test Locally

Use this when you want to test the full game on your own computer.

Start the backend:

```sh
go run .
```

The backend listens at:

```text
ws://localhost:3000
```

In another terminal, serve the frontend:

```sh
python3 -m http.server 8080
```

Then open this page in two browser tabs:

```text
http://localhost:8080
```

The page automatically uses `ws://localhost:3000` when opened from `localhost`.
Click `Enter waiting queue` in both tabs to simulate two players.

You can also run the backend tests:

```sh
go test ./...
```

## Play From GitHub Pages

Use this when you want to browse the hosted frontend or share the game with
another player. GitHub Pages hosts the static frontend, but the backend still
runs on your computer.

Start the backend and Cloudflare Tunnel:

```sh
./scripts/start-backend.sh
```

The script starts the Go backend, starts `cloudflared`, waits for a public
tunnel URL, then prints the complete frontend URL to open:

```text
https://AlanWaP.github.io/RSP/?server=wss://example.trycloudflare.com
```

Open that URL in your browser or share it with another player while the script
keeps running. The `server` query parameter tells the static frontend which live
backend tunnel to use.

By default, the script uses this GitHub Pages frontend:

```text
https://AlanWaP.github.io/RSP/
```

If you host the frontend somewhere else, set `PAGES_URL`:

```sh
PAGES_URL=https://YOUR_USERNAME.github.io/YOUR_REPO/ ./scripts/start-backend.sh
```

You can change the backend port with the `PORT` environment variable:

```sh
PORT=4000 ./scripts/start-backend.sh
```

Or pass the port as the first argument:

```sh
./scripts/start-backend.sh 4000
```

To build a reusable backend executable without Cloudflare Tunnel:

```sh
go build -o rsp-server .
./rsp-server
```

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
