# 9. Microsoft sign-in

Status: Not done.

The sign-in page has an Outlook button. The production image was built with an empty Microsoft client id, so the button does nothing.

When it is done, a Microsoft account creates the same Muster session Google creates. The assistant approve page already accepts that session. Claude does not talk to Microsoft. No second assistant protocol is required.

Still required before this spec is done:

- A Microsoft app registration whose redirect is `https://muster.asymbl.app/auth`.
- The client id present at image build time.
- A profile with no mail address must not create a blank user.
- A guest address shaped like `name_gmail.com#EXT#@tenant.onmicrosoft.com` must not be stored as the person's email.

Done when a Microsoft account can open Settings on production, and Claude sign-in can finish on that session.
