module myclouddrive-go

go 1.25.9

require (
	github.com/oapi-codegen/runtime v1.4.0
	github.com/redis/go-redis/v9 v9.7.3 // 建议检查这个，9.18 也有点超前
	gopkg.in/yaml.v3 v3.0.1
	gorm.io/driver/mysql v1.5.7 // 匹配 v1.25 的驱动
	gorm.io/gorm v1.26.0 // 改回官方最新的稳定主版本
)

require (
	gorm.io/gen v0.3.27
	gorm.io/plugin/dbresolver v1.6.2
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/tools v0.39.0 // indirect
	gorm.io/datatypes v1.2.4 // indirect
	gorm.io/hints v1.1.0 // indirect
)
