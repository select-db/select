# AI Chat

The AI chat is an in-app agent that can explore your database schemas, run queries, edit SQL files, and explain query plans.

It inherits the current user's [permissions](/docs/workspace/permissions/), so it can only access what you can.

![The chat having read a query file and proposed a rewrite: the diff open beside it with Allow and Deny, and the same choice in the chat, marked awaiting approval.](/shots/chat.light.png)

## Setup

Add your provider's API key to the workspace [`.env` file](/docs/workspace-files/env/):

| Provider   | Variable             |
| ---------- | -------------------- |
| Anthropic  | `ANTHROPIC_API_KEY`  |
| OpenAI     | `OPENAI_API_KEY`     |
| Gemini     | `GEMINI_API_KEY`     |
| OpenRouter | `OPENROUTER_API_KEY` |
| Grok (xAI) | `XAI_API_KEY`        |

The chat detects which keys are present and enables the matching providers automatically.

## Tools

The agent has access to a fixed set of tools. It cannot bypass your permissions.

### Discovery

| Tool                     | Description                                      |
| ------------------------ | ------------------------------------------------ |
| `getDatabaseSchemas`     | List schemas, tables, and views for a datasource |
| `getDatabaseTableDetail` | Get the DDL of a specific table                  |
| `readFile`               | Read a file from the workspace                   |

### Query

| Tool               | Description                                               |
| ------------------ | --------------------------------------------------------- |
| `executeQuery`     | Run a read-only SELECT                                    |
| `executeStatement` | Run a write (INSERT, UPDATE, DELETE, DDL)                 |
| `explainQuery`     | Get the execution plan for a query                        |
| `planQuery`        | Ask the planner to generate a query from natural language |

### Editing

| Tool               | Description                                                |
| ------------------ | ---------------------------------------------------------- |
| `editFile`         | Propose edits to a SQL file (shown as a diff for approval) |
| `openAgentDiffTab` | Open a diff view for the user to review changes            |

### Shell

| Tool             | Description                                                                                   |
| ---------------- | --------------------------------------------------------------------------------------------- |
| `executeCommand` | Run a shell command scoped to the workspace directory. Requires user approval before running. |

## Context

When you open the chat, it automatically picks up context from your current session:

- The active workspace and its datasources
- Databases pinned to the chat tab
- Files attached to the conversation
- The currently open file, cursor position, and text selection

This context is injected into each message so the agent can act on what you're looking at without you having to describe it.
