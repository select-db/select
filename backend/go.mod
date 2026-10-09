module backend

go 1.26.8

require (
	github.com/aws/aws-sdk-go-v2 v1.47.1
	github.com/aws/aws-sdk-go-v2/config v1.33.6
	github.com/aws/aws-sdk-go-v2/credentials v1.20.6
	github.com/aws/aws-sdk-go-v2/service/s3 v1.97.3
	github.com/benbjohnson/litestream v0.5.17
	github.com/fergusstrange/embedded-postgres v1.34.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/klauspost/compress v1.20.1
	github.com/lib/pq v1.10.9
	github.com/ovh/okms-sdk-go v0.5.4
	github.com/peterldowns/pgtestdb v0.1.1
	github.com/peterldowns/pgtestdb/migrators/goosemigrator v0.1.1
	github.com/pressly/goose/v3 v3.28.0
	github.com/selectDb/dialect v0.0.0-00010101000000-000000000000
	github.com/selectDb/toolkit v0.0.0-00010101000000-000000000000
	github.com/sqlc-dev/pqtype v0.3.0
	github.com/stretchr/testify v1.12.1
	modernc.org/sqlite v1.60.1
)

require (
	cloud.google.com/go v0.121.6 // indirect
	cloud.google.com/go/auth v0.24.0 // indirect
	cloud.google.com/go/iam v1.5.3 // indirect
	cloud.google.com/go/storage v1.56.0 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.8 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.20.18 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.10.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.38.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.43.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.1 // indirect
	github.com/aws/smithy-go v1.28.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/googleapis/gax-go/v2 v2.23.0 // indirect
	github.com/hablullah/go-hijri v1.0.2 // indirect
	github.com/hablullah/go-juliandays v1.0.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/jalaali/go-jalaali v0.0.0-20210801064154-80525e88d958 // indirect
	github.com/lmittmann/tint v1.1.3 // indirect
	github.com/magefile/mage v1.14.0 // indirect
	github.com/markusmobius/go-dateparser v1.2.4 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/oapi-codegen/runtime v1.6.0 // indirect
	github.com/prometheus/client_golang v1.17.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/psanford/sqlite3vfs v0.0.0-20260519004904-f9180fa2acc9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/superfly/ltx v0.5.2 // indirect
	github.com/tetratelabs/wazero v1.2.1 // indirect
	github.com/wasilibs/go-re2 v1.3.0 // indirect
	go.opentelemetry.io/otel/sdk v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.44.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/api v0.287.1 // indirect
	google.golang.org/genproto v0.0.0-20260319201613-d00831a3d3e7 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	modernc.org/libc v1.77.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1
	github.com/apache/arrow-go/v18 v18.8.0 // indirect
	github.com/go-sql-driver/mysql v1.10.1 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/pierrec/lz4/v4 v4.1.29 // indirect
	github.com/sethvargo/go-retry v0.4.0 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.57.0
	golang.org/x/exp v0.0.0-20260824195058-e88cd73687aa // indirect
	golang.org/x/sync v0.23.0
	golang.org/x/sys v0.48.0 // indirect
)

replace github.com/selectDb/toolkit => ../toolkit

replace github.com/selectDb/dialect => ../dialect
