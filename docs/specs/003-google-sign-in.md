# 3. Google sign-in

Status: Done.

A person can create a Muster account with Google. The browser returns to `https://muster.asymbl.app/auth`. The server stores a session cookie named `session`. That cookie is the website credential. It is not the assistant credential.

The consent screen is Asymbl Muster. The authorized domain is asymbl.app. Calendar read scopes are requested when the person connects a calendar. Contact search still asks for its own scopes.

Done when a Google account can open Settings on the production site.
