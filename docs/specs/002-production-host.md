# 2. Production host

Status: Done.

The live service is one image on the Muster droplet. Caddy terminates HTTPS and proxies to the app. `GET https://muster.asymbl.app/api/health` returns `{"ok":true}`.

The image is built for linux/amd64 and loaded onto the droplet. `MCP_ENABLED=true` is set in the server environment. Unset, `POST /mcp` returns 404.

Google's public client id is baked into the frontend at build time. The Microsoft client id is empty in the current image, so spec 9 is still open.

Done when health, the home page, and the assistant route answer on the public host.
