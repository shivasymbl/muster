# MCP sign-in for Claude

Claude's connector cannot use the website cookie. `POST /mcp` without a bearer returns 401, including `initialize`. A successful `initialize` made Claude treat the server as open. The `WWW-Authenticate` header points Claude at the discovery documents.

Claude registers itself at `POST /oauth/register`. The only accepted return addresses are `https://claude.ai/api/mcp/auth_callback` and `http://localhost` or `http://127.0.0.1` on any port, path `/callback`. Anything else is rejected.

`GET /oauth/authorize` shows an approval page when the browser already has a Muster session. If it does not, Muster sends the person to `/sign-in` and brings them back after Google or Microsoft finishes. The approve page does not care which provider created the session.

Approval mints the same `muster_` token Settings already uses, labeled Claude. A new approval revokes the previous Claude token so the five-token cap is not used up. Claude also receives a refresh token. Using it rotates both tokens. The old refresh token stops working.

A token pasted into a request header still works. This does not add a second account system.
