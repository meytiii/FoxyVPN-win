# FoxyVPN-win

**FoxyVPN-win** is an unofficial, subscription-free Firefox VPN desktop client for Windows. It utilizes Mozilla's free monthly 50 GB VPN bandwidth included with Firefox Accounts by establishing multiplexed HTTP/2 CONNECT tunnels over TLS directly to Fastly edge proxy nodes with Guardian proxy authorization tokens.

---

## Architecture & How It Works

```
                                  +-------------------------------+
                                  |    Firefox Accounts (FxA)    |
                                  |   api.accounts.firefox.com    |
                                  +---------------+---------------+
                                                  ^
                                   OAuth Token    |  PBKDF2 + HKDF authPW
                                   Hawk 2FA       |  Fastly PoW Challenge
                                                  v
+--------------------------+      +---------------+---------------+
|  Local Applications / OS | ---> |       FoxyVPN-win Core        |
|  - Edge, Chrome, Firefox |      |  - SOCKS5 (127.0.0.1:1080)    |
|  - Windows WinINet Proxy |      |  - HTTP   (127.0.0.1:8080)    |
|  - System Apps & curl    |      |  - DPI ClientHello Splitter   |
+--------------------------+      +---------------+---------------+
                                                  |
                                                  | HTTP/2 CONNECT (:443)
                                                  | Proxy-Authorization: Bearer <jwt>
                                                  v
                                  +---------------+---------------+
                                  |       Fastly Edge Node        |
                                  |  (us-nyc, de-fra, nl-ams...)  |
                                  +---------------+---------------+
                                                  |
                                                  v
                                  +-------------------------------+
                                  |       Internet / Target       |
                                  +-------------------------------+
```

### Key Technical Capabilities

1. **Firefox Accounts Control Plane (`pkg/fxa/`)**:
   - **Password Stretching**: PBKDF2 HMAC-SHA256 (1,000 rounds) using salt `identity.mozilla.com/picl/v1/quickStretch:<email>` followed by HKDF-SHA256 (`identity.mozilla.com/picl/v1/authPW`).
   - **Hawk Authentication**: HMAC-SHA256 normalized header and payload verification for session calls (`/session/verify_code`).
   - **Fastly Bot Protection Solver**: Automated proof-of-work (PoW) SHA-256 suffix search, PAT token negotiation, and client metrics generation when HTTP 406 challenges are encountered.
   - **OAuth Token Lifecycle**: Offline refresh grant persistence in `%APPDATA%\FoxyVPN\fxa_auth.json`.

2. **Guardian Proxy Pass & Quota Engine (`pkg/guardian/`)**:
   - Fetches Guardian JWT proxy pass tokens from `https://vpn.mozilla.org/api/v1/fpn/token`.
   - Live quota telemetry parsed from `X-Quota-Remaining`, `X-Quota-Limit`, and `X-Quota-Reset` response headers.
   - Autonomous token refresh loop replenishing tokens prior to expiration.

3. **Multiplexed HTTP/2 Data Plane (`pkg/upstream/`)**:
   - Raw duplex HTTP/2 CONNECT tunneling via `golang.org/x/net/http2` framing and HPACK header compression.
   - Multiplexes concurrent TCP connections over a single TLS 1.3 session to Fastly Anycast edge nodes.
   - Keepalive PING frames maintaining tunnel state across network idle periods.

4. **DPI Bypass ClientHello Splitting (`pkg/upstream/splitter.go`)**:
   - TCP segmentation of the TLS ClientHello packet (First Byte split or SNI extension split) with `TCP_NODELAY` packet flushes to defeat Deep Packet Inspection censors.

5. **Windows Routing (`pkg/winproxy/` & `pkg/proxy/`)**:
   - Local RFC 1928 SOCKS5 proxy server on `127.0.0.1:1080`.
   - Local HTTP/HTTPS CONNECT proxy server on `127.0.0.1:8080`.
   - Windows WinINet API system proxy integration using `InternetSetOptionW` (`INTERNET_OPTION_SETTINGS_CHANGED` and `INTERNET_OPTION_REFRESH`) for driverless, instant global proxy toggling.

---

## Building from Source

### Prerequisites

- **Go**: 1.24+ (tested on Go 1.25 / 1.26)
- **Node.js**: 18+ and `npm`
- **Wails CLI v2**:
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

### Build Commands

1. Clone repository:
   ```bash
   git clone https://github.com/meytiii/FoxyVPN-win.git
   cd FoxyVPN-win
   ```

2. Compile standalone Windows application:
   ```bash
   wails build
   ```
   The resulting binary will be created at:
   ```
   build\bin\FoxyVPN.exe
   ```

3. Development mode (live reload):
   ```bash
   wails dev
   ```

---

## Credits & Acknowledgements

- **Original Android Application**: Special thanks to **[Vauth](https://github.com/Vauth)** and the contributors of the original **[FoxyVPN (Android)](https://github.com/Vauth/FoxyVPN)** repository. Their reverse engineering, research into Mozilla's Firefox Accounts authentication flow, Fastly bot challenge negotiation, and Guardian proxy pass architecture provided the foundational protocol reference for this Windows desktop port.

---

## License & Disclaimer

This project is an independent open-source tool and is not affiliated with, endorsed by, or sponsored by Mozilla Corporation or Fastly. All product names, logos, and brands are property of their respective owners.
