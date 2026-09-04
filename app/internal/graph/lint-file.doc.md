# .lint File

The `.lint` file is where a workspace tunes its linting: it adds custom rules of
your own, and overrides the severity of the
[built-in rules](/docs/sql/lint-rules/). It lives at the workspace root and is
committed with everything else, so the rules your team agreed on are the rules
everyone gets.

It uses a JSON format inspired by ESLint's flat config.

> [!NOTE]
> You do not need this file to get linting. The built-in rules are on by
> default: unknown columns, ambiguous references, type mismatches, `= NULL`.
> This file is for the rules only your team knows about.

## Format

A JSON array of configuration entries. Each entry can define **custom rules**, scope them to specific **files**, or **override** rule severity.

```json .lint
[
  {
    "custom": [
      {
        "id": "no-select-star",
        "severity": "warning",
        "message": "Avoid SELECT *, list columns explicitly",
        "pattern": "(?i)\\bSELECT\\s+(?:DISTINCT\\s+|ALL\\s+)?\\*"
      },
      {
        "id": "no-drop-table",
        "severity": "error",
        "message": "DROP TABLE is not allowed in this project",
        "pattern": "(?i)DROP\\s+TABLE"
      }
    ]
  }
]
```

## Custom rules

Each custom rule has:

| Field        | Description                                           |
|--------------|-------------------------------------------------------|
| **id**       | Unique identifier for the rule                        |
| **severity** | `off`, `hint`, `warning`, or `error`                  |
| **message**  | Shown to the user when the rule matches               |
| **pattern**  | Regex pattern matched against the SQL text            |

Patterns support multiline matching and are case-sensitive by default. Use `(?i)` for case-insensitive matching.

## File scoping

Scope rules to specific files using **glob patterns**:

```json .lint
[
  {
    "custom": [
      { "id": "no-drop-table", "severity": "error", "message": "DROP TABLE not allowed", "pattern": "(?i)DROP\\s+TABLE" }
    ]
  },
  {
    "files": ["migrations/**"],
    "rules": {
      "no-drop-table": "off"
    }
  }
]
```

This defines `no-drop-table` globally, then disables it for files under `migrations/`.

**Scoping fields:**

- **files**: glob patterns that this entry applies to (e.g. `["migrations/**", "seeds/*.sql"]`)
- **ignores**: glob patterns to exclude entirely
- **rules**: override severity for existing rules by ID

Entries are applied **top to bottom**. Later entries override earlier ones for matching files.

## Overriding a built-in rule

`rules` takes any rule ID, custom or built-in. To relax
[`unquoted-uppercase`](/docs/sql/lint-rules/#predicates-and-style) under
`migrations/` while keeping it everywhere else:

```json .lint
[
  {
    "files": ["migrations/**"],
    "rules": { "unquoted-uppercase": "off" }
  }
]
```

The full list of built-in IDs is on the [Lint rules](/docs/sql/lint-rules/)
page. For a one-off exception, an inline `-- lint-disable-next-line <rule-id>`
comment is usually better than a file-wide override.

## Applying changes

After editing the `.lint` file, click **Apply** to reload lint rules. To restore the built-in defaults, click **Reset**.
