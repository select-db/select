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

## Permissions are not optional here

A local connection with no [permission](/docs/workspace/permissions/) rules is
open: the DSN is yours and you are querying your own database. A proxified one
is the opposite. The credentials are ours to hold and the query runs on our
server, so a database that no role grants access to is refused for everyone,
including the person who added it.

Adding a proxified datasource is therefore two steps: create the connection,
then give a role an allow rule for it.

## Connection pool

Proxified connections expose pool tuning:

![The lower half of a proxified connection form: the required host key, the authentication method, and the four pool settings.](/shots/dbform.proxified.light.png)

| Setting               | Default | Description                          |
|-----------------------|---------|--------------------------------------|
| **Max open conns**    | 25      | Maximum concurrent connections       |
| **Max idle conns**    | 5       | Idle connections kept in pool        |
| **Conn max lifetime** | 0       | Max reuse time in seconds (0 = none) |
| **Conn max idle time**| 0       | Max idle time in seconds (0 = none)  |

> [!WARNING]
> Setting **Max open conns** too high can exhaust your database's connection limit. Start with the default and adjust based on team size.

## Limitations

- **Environment variable substitution** (`$VAR_NAME`) is not supported in proxified mode
- Credentials are managed server-side, so local `.env` files have no effect on the connection
