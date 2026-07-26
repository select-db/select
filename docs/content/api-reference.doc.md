# API Reference

Interactive reference for the Select HTTP API. Every workspace resource is
reachable over a consistent RPC-over-POST surface; authenticate with a user
access token or an `slct_` API key in the `Authorization: Bearer <token>` header.

The reference below is generated from the live schema — the same source of truth
that drives the API itself, so it never drifts from what the server accepts.

<div id="app" style="min-height: 80vh;"></div>

<script src="/api/scalar.standalone.js"></script>

<script>
  Scalar.createApiReference('#app', {
    url: '/api/openapi.json',
  })
</script>
