# Local HTTPS with Caddy

Copy `.env.example` to `.env` and set hostnames that resolve to your Caddy
server. Set `OPENGYM_UPSTREAM` to OpenGym's existing HTTP host and port.
The defaults support local development; LAN hostnames and addresses belong
in your ignored `.env`, rather than the shared Compose or Caddy files.

Run `docker compose up -d --build`. Caddy provides HTTPS on port 443 using its
internal certificate authority. Devices need to trust that authority's root
certificate. Keep the generated `caddy-root.crt` local; it is ignored by Git.
The named `caddy-data` volume preserves Caddy's certificates across restarts.

After changing hostname or upstream environment variables, recreate Caddy:

```sh
docker compose up -d --force-recreate caddy
```
