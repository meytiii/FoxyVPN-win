# Project Guidelines & Agent Directives: FoxyVPN-win

## Core Mission
`FoxyVPN-win` is an unofficial, subscription-free Firefox VPN client for Windows. It utilizes Mozilla's free monthly VPN traffic included with Firefox accounts by establishing an HTTP/2 CONNECT tunnel over TLS directly to Fastly edge proxies with Guardian proxy tokens.

---

## Active Skills & Modes

The following skills are locally installed under `.agents/skills/` and permanently active for this project:

1. **`caveman`** (Mode: Active):
   - Ultra-compressed communication style in chat. Drops fluff, filler, articles, and unnecessary pleasantries while keeping 100% technical accuracy, exact code, exact errors, and CLI commands.
   - Note on boundaries: Content persisted outside of chat (code, comments, commit messages, documentation, `ANALYSIS.md`) must remain clear, complete, and standard professional English.

2. **`impeccable`** (Design & Craft):
   - Enforce craft floor, production-grade frontend/UI code, responsive hierarchy, intentional token usage, micro-animations, and pristine polish.
   - Ban generic, bland, or unstyled UI elements.

3. **`hallmark`** (Anti-AI-Slop Visual Systems):
   - Distinctive typography, editorial layout hierarchy, bespoke components, curated color palettes, and avoiding templated AI aesthetics.

4. **`design-taste-frontend`**:
   - Deep aesthetic sensibility, avoiding cookie-cutter cards or generic landing page tropes.

5. **`full-output-enforcement`**:
   - Zero truncated code blocks, zero placeholders (`// ...`, `// rest of code`, `etc.`).
   - Every file and deliverable must be complete, working, and fully written.

---

## Task Completion & Git Automation Directives

### Mandatory Post-Task Workflow:
For **every task completed**:
1. **Commit Summary & Description**:
   - Generate a structured Conventional Commit message (e.g. `feat: ...`, `fix: ...`, `chore: ...`, `docs: ...`).
   - Provide a concise explanation of what was changed and the rationale.
2. **Automatic Commit & Push**:
   - Stage appropriate modified/new workspace files (excluding ignored files like `ANDROID/` and `ANALYSIS.md` if ignored).
   - Commit changes to Git.
   - Automatically push to `origin/main` on remote: `https://github.com/meytiii/FoxyVPN-win`.

---

## Project Repository Architecture Rules

- **Android Reference (`ANDROID/`)**:
  - Cloned reference codebase from `https://github.com/Vauth/FoxyVPN`.
  - Must remain in `.gitignore`. Do not commit Android source trees directly to `FoxyVPN-win`.
- **Target Implementation (`FoxyVPN-win`)**:
  - Windows desktop application implementing FxA authentication, Guardian token acquisition, Netty/raw HTTP/2 CONNECT tunneling over TLS, Fastly bot challenge solving, and system proxy / Wintun routing.
