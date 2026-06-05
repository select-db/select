# Proxified Connections

By default, database credentials live in `db.config.json` on your machine. **Proxified connections** store them encrypted on the server instead.

## When to use

Use proxified connections when:

- Your **team shares databases** and credentials should not live on individual machines
- You need a **single managed connection pool** shared across developers, rather than each client opening its own connections against the database

## Enabling

Check **Proxy connection** in the connection form. The DSN and SSH fields are sent to the server and stored encrypted. The local `db.config.json` only keeps the connection name, dialect, and ID.

Once configured, **credentials never leave the server**. Users do not see or receive the DSN or SSH secrets. All database connections are established and maintained on the remote server, and queries are proxied through it.

<!-- TODO: add fixed IPs so users can allowlist our servers in their firewall rules -->

## Connection pool

Proxified connections expose pool tuning:

| Setting               | Default | Description                          |
|-----------------------|---------|--------------------------------------|
| **Max open conns**    | 25      | Maximum concurrent connections       |
| **Max idle conns**    | 5       | Idle connections kept in pool        |
| **Conn max lifetime** | 0       | Max reuse time in seconds (0 = none) |
| **Conn max idle time**| 0       | Max idle time in seconds (0 = none)  |

> Setting **Max open conns** too high can exhaust your database's connection limit. Start with the default and adjust based on team size.

## Limitations

- **Environment variable substitution** (`$VAR_NAME`) is not supported in proxified mode
- Credentials are managed server-side, so local `.env` files have no effect on the connection
