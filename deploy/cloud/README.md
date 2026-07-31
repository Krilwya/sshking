# SSHKing Cloud deployment

The cloud service stores account identity, team/server metadata, device sessions,
and stable tab/tmux resume handles. SSH private keys, passwords, biometric
preferences, and local identity paths remain on each device, and SSH connections
continue directly from the desktop app to the target.

Build `cmd/sshking-api` for Linux as `sshking-api`, place it beside the
Dockerfile, copy `.env.example` to `.env`, then run:

```sh
docker compose up -d --build
curl http://127.0.0.1:8787/healthz
```

The API joins the existing `nginx-reverse_default` network as `sshking-api:8080`
and only publishes its diagnostic endpoint on VPS localhost. Configure an HTTPS
proxy host before setting `SSHKING_PUBLIC_URL` or enabling OAuth providers.

Provider callback URLs are listed in `.env.example`. Apple expects a Service ID
and a generated client-secret JWT; rotate that JWT before it expires.
