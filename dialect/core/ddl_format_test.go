package core

import "testing"

func TestFormatTableDDL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "align columns",
			input: `CREATE TABLE users (
  id text NOT NULL,
  name varchar(255),
  email text NOT NULL DEFAULT '',
);`,
			expected: `CREATE TABLE users (
  id    text         NOT NULL,
  name  varchar(255),
  email text         NOT NULL DEFAULT '',
);`,
		},
		{
			name:     "empty DDL",
			input:    "",
			expected: "",
		},
		{
			name:     "no CREATE TABLE",
			input:    "SELECT * FROM users;",
			expected: "SELECT * FROM users;",
		},
		{
			name: "quoted identifiers",
			input: `CREATE TABLE t (
  "id" text,
  "userName" varchar(100),
);`,
			expected: `CREATE TABLE t (
  "id"       text,
  "userName" varchar(100),
);`,
		},
		{
			name: "complex types",
			input: `CREATE TABLE t (
  id text,
  "createdAt" timestamp(3) without time zone NOT NULL,
  status "AppAccountStatus" NOT NULL,
);`,
			expected: `CREATE TABLE t (
  id          text,
  "createdAt" timestamp(3) without time zone NOT NULL,
  status      "AppAccountStatus"             NOT NULL,
);`,
		},
		{
			name: "real world example",
			input: `CREATE TABLE backup."AppAccount_Old" (
  id text NOT NULL,
  "createdAt" timestamp(3) without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" timestamp(3) without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "collaboratorId" text NOT NULL,
  "deletedAt" timestamp(3) without time zone,
  status "AppAccountStatus" NOT NULL DEFAULT 'active'::"AppAccountStatus",
  senders text[] DEFAULT ARRAY[]::text[],
);`,
			expected: `CREATE TABLE backup."AppAccount_Old" (
  id               text                           NOT NULL,
  "createdAt"      timestamp(3) without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt"      timestamp(3) without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "collaboratorId" text                           NOT NULL,
  "deletedAt"      timestamp(3) without time zone,
  status           "AppAccountStatus"             NOT NULL DEFAULT 'active'::"AppAccountStatus",
  senders          text[]                         DEFAULT ARRAY[]::text[],
);`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTableDDL(tt.input)
			if result != tt.expected {
				t.Errorf("FormatTableDDL()\ngot:  %q\nwant: %q", result, tt.expected)
			}
		})
	}
}
