# Lint File

The `.lint` file defines custom SQL linting rules for your workspace. It lives at the workspace root and uses a JSON format inspired by ESLint's flat config.

## Format

A JSON array of configuration entries. Each entry can define **custom rules**, scope them to specific **files**, or **override** rule severity.

```json
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

```json
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

## Applying changes

After editing the `.lint` file, click **Apply** to reload lint rules. To restore the built-in defaults, click **Reset**.
