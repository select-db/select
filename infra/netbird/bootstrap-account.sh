#!/usr/bin/env bash
# Configure the NetBird Cloud account: groups, access policies, setup keys.
# Idempotent, and the source of truth for all three. See netbird-ssh.doc.md.
#
# Requires: NETBIRD_TOKEN (a personal access token with admin rights), curl, jq.
#
# Usage:
#   NETBIRD_TOKEN=nbp_... ./bootstrap-account.sh
#
# Progress goes to stderr. Newly created setup keys go to stdout, one per line,
# as "<name><tab><key>". They are shown once by the API and never again.

set -euo pipefail

NETBIRD_API=${NETBIRD_API:-https://api.netbird.io/api}

# Server keys last a year; CI keys a month, since CI peers are ephemeral and
# a leaked key with no host to enroll is still a way into the network.
SERVER_KEY_TTL=${SERVER_KEY_TTL:-31536000}
CI_KEY_TTL=${CI_KEY_TTL:-2592000}

# The CI policy is created disabled: nothing deploys from CI yet, and a policy
# that grants access to a runner nobody uses is a standing hole.
CI_POLICY_ENABLED=${CI_POLICY_ENABLED:-false}

for cmd in curl jq; do
  command -v "$cmd" >/dev/null || {
    echo "error: $cmd is required" >&2
    exit 1
  }
done

if [[ -z ${NETBIRD_TOKEN:-} ]]; then
  echo "error: NETBIRD_TOKEN is not set" >&2
  exit 1
fi

api() {
  local method=$1 path=$2 payload=${3:-} response status
  if [[ -n $payload ]]; then
    response=$(curl -sS -w $'\n%{http_code}' -X "$method" "$NETBIRD_API$path" \
      -H "Authorization: Token $NETBIRD_TOKEN" \
      -H 'Content-Type: application/json' \
      -d "$payload")
  else
    response=$(curl -sS -w $'\n%{http_code}' -X "$method" "$NETBIRD_API$path" \
      -H "Authorization: Token $NETBIRD_TOKEN")
  fi
  status=${response##*$'\n'}
  response=${response%$'\n'*}
  if [[ $status != 2* ]]; then
    echo "error: $method $path returned $status" >&2
    echo "$response" >&2
    return 1
  fi
  printf '%s' "$response"
}

group_id() {
  local name=$1 id
  id=$(api GET /groups | jq -r --arg n "$name" 'map(select(.name == $n)) | .[0].id // empty')
  if [[ -z $id ]]; then
    id=$(api POST /groups "$(jq -n --arg n "$name" '{name: $n}')" | jq -r '.id')
    echo "created group $name" >&2
  fi
  printf '%s' "$id"
}

upsert_policy() {
  local name=$1 payload=$2 id
  id=$(api GET /policies | jq -r --arg n "$name" 'map(select(.name == $n)) | .[0].id // empty')
  if [[ -n $id ]]; then
    api PUT "/policies/$id" "$payload" >/dev/null
    echo "updated policy $name" >&2
  else
    api POST /policies "$payload" >/dev/null
    echo "created policy $name" >&2
  fi
}

# NetBird ships an enabled "Default" policy that lets every peer reach every
# other peer, which would give a compromised staging host a path to prod.
disable_default_policy() {
  local policy id payload
  policy=$(api GET /policies | jq -c 'map(select(.name == "Default" and .enabled)) | .[0] // empty')
  [[ -n $policy ]] || return 0
  id=$(jq -r '.id' <<<"$policy")
  payload=$(jq -c '{
    name: .name,
    description: (.description // ""),
    enabled: false,
    rules: [.rules[] | {
      name: .name,
      description: (.description // ""),
      enabled: false,
      action: .action,
      bidirectional: .bidirectional,
      protocol: .protocol,
      sources: [(.sources // [])[].id],
      destinations: [(.destinations // [])[].id]
    } + (if (.ports // []) | length > 0 then {ports: .ports} else {} end)]
  }' <<<"$policy")
  api PUT "/policies/$id" "$payload" >/dev/null
  echo "disabled the default allow-all policy" >&2
}

ssh_policy() {
  local name=$1 description=$2 enabled=$3 source=$4
  shift 4
  jq -n \
    --arg name "$name" \
    --arg description "$description" \
    --argjson enabled "$enabled" \
    --arg source "$source" \
    --argjson destinations "$(printf '%s\n' "$@" | jq -R . | jq -sc .)" \
    '{
      name: $name,
      description: $description,
      enabled: $enabled,
      rules: [{
        name: $name,
        description: $description,
        enabled: $enabled,
        action: "accept",
        bidirectional: false,
        protocol: "tcp",
        ports: ["22"],
        sources: [$source],
        destinations: $destinations
      }]
    }'
}

# A key is left alone once it exists: rotating one means revoking it in the
# dashboard, which is a decision with a re-enrollment attached.
ensure_setup_key() {
  local name=$1 group=$2 ephemeral=$3 ttl=$4 existing payload key
  existing=$(api GET /setup-keys |
    jq -r --arg n "$name" 'map(select(.name == $n and .revoked == false)) | .[0].id // empty')
  if [[ -n $existing ]]; then
    echo "setup key $name exists, left alone" >&2
    return 0
  fi
  payload=$(jq -n \
    --arg name "$name" \
    --arg group "$group" \
    --argjson ephemeral "$ephemeral" \
    --argjson ttl "$ttl" \
    '{
      name: $name,
      type: "reusable",
      expires_in: $ttl,
      auto_groups: [$group],
      usage_limit: 0,
      ephemeral: $ephemeral
    }')
  key=$(api POST /setup-keys "$payload" | jq -r '.key')
  echo "created setup key $name" >&2
  printf '%s\t%s\n' "$name" "$key"
}

admins=$(group_id admins)
prod=$(group_id srv-prod)
staging=$(group_id srv-staging)
ci=$(group_id ci)

disable_default_policy

upsert_policy admins-ssh-servers \
  "$(ssh_policy admins-ssh-servers "Maintainers reach the fleet over SSH" true "$admins" "$prod" "$staging")"
upsert_policy ci-ssh-servers \
  "$(ssh_policy ci-ssh-servers "Deploy runners reach the fleet over SSH" "$CI_POLICY_ENABLED" "$ci" "$prod" "$staging")"

ensure_setup_key srv-prod "$prod" false "$SERVER_KEY_TTL"
ensure_setup_key srv-staging "$staging" false "$SERVER_KEY_TTL"
ensure_setup_key ci "$ci" true "$CI_KEY_TTL"

echo "done: store any key printed above in the KMS secret store" >&2
