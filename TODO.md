# TODO

## Phase 1: Core IRC Server

-   [x] Initialize Go module and basic TCP server skeleton
-   [x] Accept client connections and log connection details
-   [x] Parse and handle basic RFC 1459 commands (NICK, USER, QUIT, JOIN, PART, PRIVMSG, NOTICE, PING, PONG)
-   [x] Maintain an in-memory registry of connected clients

## Phase 2: Channels

-   [x] Create a Channel struct (name, topic, connected clients)
-   [x] Implement JOIN and PART logic
-   [x] Track user membership in channels

## Phase 3: Persistence

-   [x] Integrate SQLite for storing users, channels, and logs
-   [x] Implement database schema creation
-   [x] Store user/channel data and log messages (timestamp, sender, recipient, content)

## Phase 4: Logging

-   [x] Create a dedicated logging module
-   [x] Log connect, disconnect, join, part, message events
-   [x] Output to console and persist to SQLite

## Phase 5: Web Interface

-   [x] Serve a dashboard showing active users, channels, messages
-   [ ] Allow sending messages through the web
-   [x] Optionally integrate basic real-time updates (polling or websockets)

## Phase 6: Configuration

-   [ ] Load YAML config (server_name, host, port, etc.)
-   [ ] Replace hardcoded values with config data
-   [ ] Provide sane defaults for missing/invalid config

## Phase 7: Final Integration

-   [ ] Ensure all components (IRC server, persistence, web interface, logging, config) work together
-   [ ] Implement graceful shutdown (close DB, cleanup resources)
-   [ ] Document usage and configuration in README
-   [ ] Test with basic IRC clients/bots to ensure RFC 1459 compliance
