# IRC Server

A modern IRC server implementation in Go, following RFC 1459 specifications.

## Features

-   Full IRC protocol support (RFC 1459)
-   Web interface for monitoring
-   SQLite-based persistence
-   YAML configuration
-   Graceful shutdown
-   Channel support
-   Private messaging
-   User authentication

## Installation

```bash
go get github.com/yourusername/ircserver
```

## Configuration

Create a `config.yaml` file:

```yaml
server:
    name: "My IRC Server"
    host: "localhost"
    port: "6667"
    web_port: "8080"

storage:
    log_path: "irc.log"
    sqlite_path: "irc.db"

irc:
    default_channel: "#general"
    max_message_length: 512
```

## Usage

Start the server:

```bash
./ircserver -config config.yaml
```

Available flags:

-   `-config`: Path to config file (default: config.yaml)
-   `-host`: Override server host from config
-   `-port`: Override server port from config
-   `-web-port`: Override web interface port from config
-   `-version`: Print version and exit

## Connecting

Use any standard IRC client to connect to the server:

```bash
# Using netcat for testing
nc localhost 6667

# Using a GUI client
# Server: localhost
# Port: 6667
```

The web interface is available at `http://localhost:8080`

---

We like Halloy. It is a very nice rust IRC client. Here is an example halloy config that will connect to the server easily:

```toml
[servers.localhost]
nickname = "harper"
server = "localhost"
port = 6667
use_tls = false
channels = ["#2389"]
```

## Development

Build:

```bash
go build -o ircserver cmd/ircserver/main.go
```

Run tests:

```bash
go test ./...
```

## License

MIT License
