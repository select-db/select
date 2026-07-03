module github.com/selectDb/dialect

go 1.25.11

require (
	github.com/antlr4-go/antlr/v4 v4.13.1
	github.com/apache/arrow-go/v18 v18.6.0
	github.com/go-sql-driver/mysql v1.10.0
	github.com/klauspost/compress v1.19.0
	github.com/lib/pq v1.10.9
	github.com/selectDb/toolkit v0.0.0-00010101000000-000000000000
	golang.org/x/crypto v0.52.0
	modernc.org/sqlite v1.52.0
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	golang.org/x/exp v0.0.0-20260112195511-716be5621a96 // indirect
	golang.org/x/sys v0.45.0 // indirect
	modernc.org/libc v1.72.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

replace github.com/selectDb/toolkit => ../toolkit
