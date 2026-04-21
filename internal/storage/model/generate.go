//go:generate gentool -dsn "myclouddrive:myclouddrive@tcp(127.0.0.1:3306)/myclouddrive?charset=utf8mb4&parseTime=True&loc=Local" -tables "storage_platform,storage_settings" -outPath "./gen" -modelPkgName "model"
package model
