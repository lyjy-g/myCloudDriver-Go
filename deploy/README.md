修改后重建数据库的命令是什么

docker-compose down -v

-v → 删除 Compose 文件中定义的 所有卷（mysql_data、redis_data、minio_data、app_data）。