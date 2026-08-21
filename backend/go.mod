module backend

go 1.25.13

require (
	github.com/fergusstrange/embedded-postgres v1.34.0
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/klauspost/compress v1.19.1
	github.com/lib/pq v1.10.9
	github.com/ovh/okms-sdk-go v0.5.3
	github.com/peterldowns/pgtestdb v0.1.1
	github.com/peterldowns/pgtestdb/migrators/goosemigrator v0.1.1
	github.com/pressly/goose/v3 v3.27.3
	github.com/selectDb/dialect v0.0.0-00010101000000-000000000000
	github.com/selectDb/toolkit v0.0.0-00010101000000-000000000000
	github.com/sqlc-dev/pqtype v0.3.0
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/oapi-codegen/runtime v1.4.1 // indirect
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/apache/arrow-go/v18 v18.7.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-sql-driver/mysql v1.10.0 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/pierrec/lz4/v4 v4.1.27 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/sethvargo/go-retry v0.4.0 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.55.0
	golang.org/x/exp v0.0.0-20260718201538-764159d718ef // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/selectDb/toolkit => ../toolkit

replace github.com/selectDb/dialect => ../dialect
