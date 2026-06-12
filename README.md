# Jitter — Real-Time WebRTC Video Conferencing

[![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![WebRTC](https://img.shields.io/badge/WebRTC-Enabled-333333?logo=webrtc&logoColor=white)](https://webrtc.org/)
[![Docker](https://img.shields.io/badge/Docker-Multi--stage-2496ED?logo=docker&logoColor=white)](https://www.docker.com/)
[![Caddy](https://img.shields.io/badge/Caddy-Reverse_Proxy-1f8197?logo=caddy&logoColor=white)](https://caddyserver.com/)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

<div align="center">
  <img src="demo.gif" width="100%" alt="Jitter demo — WebRTC calls in action">
  <p><i>Secure, peer-to-peer voice and video calls powered by WebRTC and Go.</i></p>
</div>

---

## 📖 Overview

**Jitter** is a lightweight, high-performance video conferencing application designed to demonstrate the power of WebRTC combined with a concurrent Go backend. It avoids routing media traffic through central servers, establishing direct peer-to-peer connections for minimal latency. 

This project is built with production readiness in mind, featuring **Dockerized multi-stage builds**, **Caddy-automated HTTPS**, and a self-hosted **Coturn server** for NAT traversal.

---

## ✨ Key Features & Technical Highlights

- **🎥 Peer-to-Peer Media (WebRTC):** Direct audio/video streams between clients (using `RTCPeerConnection` and `MediaStream` APIs) without intermediate media servers.
- **⚡ Concurrent Signaling Server (Go):** A highly concurrent WebSocket hub built with Go and Gin. Utilizes goroutines, channels, and mutexes to safely route SDP offers/answers and ICE candidates in real-time.
- **🛡️ Secure NAT Traversal:** Integrates a self-hosted Coturn (STUN/TURN) server to ensure reliable connections across strict firewalls, symmetric NATs, and cellular networks.
- **🔒 Production-Grade Infrastructure:** 
  - **Multi-stage Docker builds:** Generates a minimal, static Alpine Linux binary container (`CGO_ENABLED=0`), reducing attack surface and image size (~5MB).
  - **Caddy Reverse Proxy:** Automatically provisions and renews TLS certificates via Let's Encrypt for secure WSS/HTTPS connections.
  - **Least Privilege:** Containers run under non-root users (`jitter`) with read-only filesystems and dropped capabilities (`cap_drop: ALL`).
- **🗄️ Type-Safe Database:** Uses PostgreSQL to validate and manage unique room UUIDs. Go data access layers are auto-generated via **sqlc**, guaranteeing compile-time SQL safety.
- **📱 PWA & Mobile UX:** Fully responsive, installable Progressive Web App. Includes mobile-specific features like dynamic track replacement (`replaceTrack`) for switching between front and back cameras without dropping the connection.
- **🛠️ Resilient Client State:** Implements ICE candidate buffering and race-condition mitigations to handle asynchronous camera permission requests gracefully.

---

## 📐 Architecture

```text
┌───────────────┐        WebSocket (wss://)         ┌───────────────────┐
│               │◄─────────────────────────────────►│                   │
│   Client A    │         Signaling Traffic         │    Go Backend     │
│   (Laptop)    │      (SDP Offer/Answer, ICE)      │    (Gin + WS)     │
│               │                                   │                   │
└───────┬───────┘                                   └───┬───────────┬───┘
        │                                               │           │
        │                                               │           │ SQL (pgx)
        │      WebRTC Peer-to-Peer Media                │           │
        │◄───────────────────────────────────►  ┌───────▼────────┐  │
        │    (Direct or relayed via Coturn)     │                │  │
        │                                       │     Caddy      │  │
        │                                       │ (Auto HTTPS)   │  │
┌───────▼───────┐                               │                │  │
│               │                               └────────────────┘  │
│   Client B    │                                ┌──────────────────▼──┐
│   (Mobile)    │                                │     PostgreSQL      │
│               │                                │    (Room UUIDs)     │
└───────────────┘                                └─────────────────────┘
```

---

## 🚀 Deployment Guide (Production)

The production environment is orchestrated using Docker Compose.

### Prerequisites
- A remote server (VPS) with Docker and Docker Compose installed.
- A domain name pointing (A record) to the server's public IP.

### 1. Clone the repository
```bash
git clone https://github.com/zkqw3r/Jitter.git
cd Jitter
```

### 2. Configure Environment Variables
Create a `.env` file based on `.env.example`:
```bash
cp .env.example .env
nano .env
```
Ensure you fill in your real domain and secure passwords:
```env
# Database
POSTGRES_PASSWORD=your_secure_password
DATABASE_URL=postgres://jitter:your_secure_password@db:5432/jitter?sslmode=disable

# Domain & WebRTC Settings
DOMAIN=yourdomain.com
ALLOWED_ORIGINS=https://yourdomain.com
TURN_SECRET=your_random_secret_string
TURN_REALM=yourdomain.com
TURN_EXTERNAL_IP=your_server_public_ip

# Public STUN/TURN routes
STUN_URL=stun:stun.l.google.com:19302
TURN_URL_UDP=turn:yourdomain.com:3478
TURN_URL_TCP=turn:yourdomain.com:3478
```

### 3. Open Firewall Ports
Jitter requires standard web ports and Coturn UDP/TCP ports:
```bash
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 3478/udp
ufw allow 3478/tcp
```

### 4. Build and Start
Run the multi-stage build and start the cluster in detached mode:
```bash
docker compose -f docker-compose.prod.yml up --build -d
```
Caddy will automatically fetch an SSL certificate, and your secure WebRTC app will be live at `https://yourdomain.com`.

---

## 🛠️ Local Development

If you want to run the project locally for development:

1. Copy `.env.example` to `.env`.
2. Start the development infrastructure:
   ```bash
   docker compose up db turn -d
   ```
3. Generate SQL queries (if modified) and install Go dependencies:
   ```bash
   sqlc generate
   go mod tidy
   ```
4. Run the Go backend:
   ```bash
   cd backend
   go run ./cmd/server
   ```
5. Open `http://localhost:8080` in your browser.

---

## 📁 Project Structure

```
Jitter/
├── backend/
│   ├── cmd/server/main.go       # Application entrypoint
│   ├── internal/
│   │   ├── config/              # Environment loading
│   │   ├── db/                  # SQL migrations & sqlc generated code
│   │   ├── handler/             # HTTP endpoints and WS upgrade
│   │   └── signaling/           # WebSocket Hub & Client goroutine logic
├── frontend/
│   ├── app.js                   # WebRTC negotiation, MediaStreams, UI state
│   ├── call.html / index.html   # HTML templates
│   ├── style.css                # Mobile-first CSS UI
│   └── manifest.json            # PWA manifest
├── docker/                      # External config (Coturn, init.sql)
├── Dockerfile.prod              # Multi-stage static build instructions
├── docker-compose.prod.yml      # Production stack (App, DB, Caddy, TURN)
└── README.md
```

---

## 📄 License
This project is licensed under the [MIT License](LICENSE).

<div align="center">
  <sub>Made with ❤️ by <b>zkqw3r</b></sub>
</div>
