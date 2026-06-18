# Multiplayer Rock Paper Scissors

A small two-player Rock Paper Scissors game. The browser UI is a static site
that can be hosted on GitHub Pages, while the realtime matchmaking backend runs
as a lightweight Go WebSocket server on your PC.

## How It Works

GitHub Pages only serves static files. It hosts:

- `index.html`
- `style.css`
- `script.js`

The backend runs separately from GitHub Pages:

- `main.go` starts a Go WebSocket server.
- Players connect from the page to the backend.
- The backend puts players into a waiting queue.
- When two players are waiting, the backend creates a game room.
- Each browser submits one move.
- The backend calculates the result and sends it to both browsers.

The browser page can connect to any backend URL you provide. During local
testing that URL is usually `ws://localhost:3000`. From GitHub Pages it must be
a secure `wss://` URL.

## Run The Backend Locally

Install Go, then download dependencies:

```sh
go mod tidy
```

Start the backend on your PC:

```sh
./scripts/start-backend.sh
```

By default the backend listens here:

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

To build a reusable backend executable:

```sh
go build -o rsp-server .
./rsp-server
```

## Test Locally

For a quick test, open `index.html` directly in two browser tabs. If that is
blocked by browser file restrictions, serve the directory with a simple static
server:

```sh
python3 -m http.server 8080
```

Then open:

```text
http://localhost:8080
```

The page will automatically use `ws://localhost:3000` when opened from
`localhost`. Open it in two tabs or two browsers to simulate two players.

You can also run the backend tests:

```sh
go test ./...
```

## Expose Your PC Backend

When the frontend is on GitHub Pages, other players need a public HTTPS/WSS URL
that reaches the backend on your PC. A tunnel is the easiest way to do this.

Cloudflare Tunnel example:

```sh
cloudflared tunnel --url http://localhost:3000
```

ngrok example:

```sh
ngrok http 3000
```

The tunnel tool will print a public HTTPS URL. Convert that URL to WebSocket by
using `wss://` in the game page. For example:

```text
https://example-tunnel.trycloudflare.com
```

becomes:

```text
wss://example-tunnel.trycloudflare.com
```

Keep your PC awake and keep both `./scripts/start-backend.sh` and the tunnel running while people
are playing. If either process stops, players will disconnect.

## Deploy The Frontend To GitHub Pages

Create a GitHub repository and push this project:

```sh
git init
git add .
git commit -m "Add multiplayer rock paper scissors"
git branch -M main
git remote add origin https://github.com/YOUR_USERNAME/YOUR_REPO.git
git push -u origin main
```

Enable GitHub Pages:

1. Open the repository on GitHub.
2. Go to `Settings` > `Pages`.
3. Under `Build and deployment`, choose `Deploy from a branch`.
4. Select branch `main` and folder `/root`.
5. Save and wait for GitHub to publish the site.

Your frontend URL will look like:

```text
https://YOUR_USERNAME.github.io/YOUR_REPO/
```

Open the page with the backend URL as a query parameter:

```text
https://YOUR_USERNAME.github.io/YOUR_REPO/?server=wss://YOUR_TUNNEL_URL
```

You can also paste the backend WebSocket URL into the page. The browser stores
the most recent backend URL in local storage.

## Study-Only Node Backend

The original Node.js backend was moved to `study/node-backend/` for study and
comparison only. The active backend is the Go server in `main.go`.

To run the study copy:

```sh
cd study/node-backend
npm install
npm start
```

## Important Notes

- GitHub Pages cannot run the backend. It only hosts the static browser files.
- The backend currently stores games in memory, so all games reset when the
  server restarts.
- Do not expose this as a serious public service from your PC without thinking
  about security, rate limiting, and uptime.
- For casual testing with friends, a local backend plus Cloudflare Tunnel or
  ngrok is enough.
