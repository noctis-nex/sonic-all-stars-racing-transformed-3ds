# Sonic & All-Stars Racing Transformed (3DS) server

A community NEX/PRUDP server implementation for the Nintendo 3DS version of *Sonic & All-Stars Racing Transformed*. This repository contains server code, not game code or game assets. It is not affiliated with or endorsed by Nintendo or SEGA.

The title is listed in [Nintendo’s 3DS game catalogue](https://www.nintendo.com/en-gb/Games/Nintendo-3DS-games/Sonic-All-Stars-Racing-Transformed-688963.html).

## Status

This is an experimental implementation. The code registers ticket granting, secure connection, NAT traversal, matchmaking, matchmaking extensions, and logs unknown protocol IDs. That describes what the server currently registers; it does not establish that every game feature or packet format works with an unmodified console. No end-to-end compatibility claim is made here.

The NEX library version defaults to `2.0.0` and can be overridden with `PN_SONIC3DS_NEX_VERSION`. The game-specific NEX version, protocol behavior, and deployment requirements should be verified against real client traffic before relying on this server.

## Requirements

- Go 1.24 or newer
- PostgreSQL reachable by the server
- A Protarium account gRPC service reachable by the server
- Network routing for the configured NEX UDP ports
- Game-specific access key supplied through configuration; it is intentionally not included in this repository

The common NEX protocol library creates its required database tables when its protocols are initialized. Review the upstream implementation and your deployment’s database permissions before running it.

## Configuration

Copy `.env.example` to `.env`, then replace every example value. The application exits if the database URI, authentication and secure server ports, or account gRPC host and port are missing. The account API key and game access key must also be configured for a working deployment.

| Variable | Purpose |
| --- | --- |
| `PN_SONIC3DS_POSTGRES_URI` | PostgreSQL connection URI |
| `PN_SONIC3DS_AUTHENTICATION_SERVER_PORT` | UDP authentication server port |
| `PN_SONIC3DS_SECURE_SERVER_HOST` | Publicly reachable host advertised to clients |
| `PN_SONIC3DS_SECURE_SERVER_PORT` | UDP secure server port |
| `PN_SONIC3DS_ACCOUNT_GRPC_HOST` | Account service host |
| `PN_SONIC3DS_ACCOUNT_GRPC_PORT` | Account service port |
| `PN_SONIC3DS_ACCOUNT_GRPC_API_KEY` | Account service API key |
| `PN_SONIC3DS_ACCESS_KEY` | Game-specific NEX access key, supplied by the operator |
| `PN_SONIC3DS_NEX_VERSION` | Optional NEX version in `major.minor.patch` form; current default is `2.0.0` |
| `PN_SONIC3DS_VERBOSE_PACKET_LOGGING` | Optional extra protocol logging when set to `1` |

The account gRPC client in this code uses plaintext transport credentials. Keep that connection on a trusted private network or protect it with an authenticated encrypted tunnel; do not expose it directly to the public Internet.

## Build and run

```sh
go test ./...
go build ./...
go run .
```

The process loads `.env` from its working directory. The server listens for NEX/PRUDP traffic on the configured UDP ports. Configure firewall rules and the advertised secure host for your own network.

## Logging and privacy

The server logs protocol IDs, method IDs, and player PIDs. Some registered callbacks also log station URLs, matchmaking criteria, and NAT traversal metadata. Treat production logs as potentially identifying data and restrict their access and retention. Verbose packet logging is optional; packet bodies are not intentionally dumped by the observer in this repository.

## Dependencies and licenses

The top-level server is distributed under GNU AGPL v3; see [LICENSE](LICENSE). The `nex-go-patch` and `nex-common-patch` directories contain modified copies of PretendoNetwork modules, with their upstream license files and README notices retained. Both upstream repositories identify AGPL-3.0 licensing: [nex-go](https://github.com/PretendoNetwork/nex-go), [nex-protocols-common-go](https://github.com/PretendoNetwork/nex-protocols-common-go), and [nex-protocols-go](https://github.com/PretendoNetwork/nex-protocols-go). Other dependencies are fetched by Go from the versions recorded in `go.mod` and `go.sum`; consult each dependency’s license before redistribution outside the terms that apply to it.

See [NOTICE.md](NOTICE.md) for source attribution details. This project does not contain Nintendo or SEGA game binaries, assets, or source code.
