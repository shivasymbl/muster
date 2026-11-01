# 8. Claude sign-in

Status: Done.

Claude's connector cannot hold the website cookie. Sign in now is a separate approval in front of the same account.

Claude registers at `POST /oauth/register`. The only accepted return addresses are `https://claude.ai/api/mcp/auth_callback` and a loopback `/callback` on localhost or 127.0.0.1. Any other redirect is rejected.

If the browser has no Muster session, the approve page sends the person to the website sign-in and brings them back. Google works today. Microsoft uses the same return once spec 9 is done.

Approval mints a `muster_` token labeled Claude. A later approval replaces that token. Claude also receives a refresh token. Using it rotates both. A pasted `Authorization` header still works for clients that cannot do this dance.

`initialize` without a bearer returns 401. A successful initialize with no token made Claude treat the server as public.

Done when Claude detects sign-in, the person approves, and a later tool call runs as that account.
