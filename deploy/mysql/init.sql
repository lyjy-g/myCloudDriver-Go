
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- Table structure for file_info
-- ----------------------------
DROP TABLE IF EXISTS `file_info`;
CREATE TABLE `file_info` (
                             `id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '文件ID',
                             `object_key` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '对象存储Key/资源名称',
                             `original_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '资源原始名称',
                             `display_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '资源显示名称',
                             `suffix` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '后缀名',
                             `size` bigint NOT NULL DEFAULT 0 COMMENT '大小(字节)',
                             `mime_type` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '标准MIME类型',
                             `is_dir` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否目录 0否 1是',
                             `parent_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '父节点ID',
                             `user_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '用户ID',
                             `workspace_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '工作空间ID',
                             `content_md5` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '文件MD5，用于秒传和文件校验',
                             `storage_platform_setting_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '存储配置ID',
                             `upload_time` datetime NOT NULL COMMENT '上传时间',
                             `update_time` datetime DEFAULT NULL COMMENT '修改时间',
                             `last_access_time` datetime DEFAULT NULL COMMENT '最后访问时间',
                             `is_deleted` tinyint(1) NOT NULL DEFAULT 0 COMMENT '软删除标记 0未删除 1已删除',
                             `deleted_time` datetime DEFAULT NULL COMMENT '删除时间',
                             PRIMARY KEY (`id`) USING BTREE,
                             KEY `idx_recycle_query` (`workspace_id`, `user_id`, `storage_platform_setting_id`, `is_deleted`, `parent_id`) USING BTREE,
                             KEY `idx_workspace_parent` (`workspace_id`, `parent_id`) USING BTREE,
                             KEY `idx_user_parent` (`user_id`, `parent_id`) USING BTREE,
                             KEY `idx_content_md5` (`content_md5`) USING BTREE,
                             KEY `idx_storage_setting_id` (`storage_platform_setting_id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='文件资源表' ROW_FORMAT=DYNAMIC;

-- ----------------------------
-- Table structure for share_info
-- ----------------------------
DROP TABLE IF EXISTS `share_info`;
CREATE TABLE `share_info` (
                              `id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '分享ID',
                              `user_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '分享人ID',
                              `workspace_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '归属工作空间ID',
                              `storage_setting_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '归属存储配置ID',
                              `share_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '分享名称',
                              `share_code` varchar(6) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '提取码（可为空）',
                              `expire_time` datetime DEFAULT NULL COMMENT '过期时间（NULL表示永久有效）',
                              `scope` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '权限范围: preview,download（逗号分隔）',
                              `view_count` int NOT NULL DEFAULT 0 COMMENT '查看次数统计',
                              `max_view_count` int DEFAULT NULL COMMENT '最大查看次数（NULL表示无限制）',
                              `download_count` int NOT NULL DEFAULT 0 COMMENT '下载次数统计',
                              `max_download_count` int DEFAULT NULL COMMENT '最大下载次数（NULL表示无限制）',
                              `created_at` datetime NOT NULL COMMENT '创建时间',
                              `updated_at` datetime NOT NULL COMMENT '更新时间',
                              PRIMARY KEY (`id`) USING BTREE,
                              KEY `idx_user_id` (`user_id`) USING BTREE,
                              KEY `idx_workspace_id` (`workspace_id`) USING BTREE,
                              KEY `idx_workspace_setting` (`workspace_id`, `storage_setting_id`) USING BTREE,
                              KEY `idx_expire_time` (`expire_time`) USING BTREE,
                              KEY `idx_share_code` (`share_code`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='文件分享表' ROW_FORMAT=DYNAMIC;

-- ----------------------------
-- Table structure for share_access_record
-- ----------------------------
DROP TABLE IF EXISTS `share_access_record`;
CREATE TABLE `share_access_record` (
                                       `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                       `share_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '分享ID',
                                       `access_ip` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '访问IP',
                                       `access_address` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '访问地址',
                                       `browser` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '浏览器类型',
                                       `os` varchar(512) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '操作系统',
                                       `access_time` datetime NOT NULL COMMENT '访问时间',
                                       PRIMARY KEY (`id`) USING BTREE,
                                       KEY `idx_share_access_time` (`share_id`, `access_time`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='分享页面访问记录表' ROW_FORMAT=DYNAMIC;

-- ----------------------------
-- Table structure for share_items
-- ----------------------------
DROP TABLE IF EXISTS `share_items`;
CREATE TABLE `share_items` (
                               `share_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '分享ID',
                               `file_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '文件/文件夹ID',
                               `created_at` datetime NOT NULL COMMENT '创建时间',
                               PRIMARY KEY (`share_id`, `file_id`) USING BTREE,
                               KEY `idx_file_id` (`file_id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='分享文件关联表' ROW_FORMAT=DYNAMIC;

-- ----------------------------
-- Table structure for file_transfer_task
-- ----------------------------
DROP TABLE IF EXISTS `file_transfer_task`;
CREATE TABLE `file_transfer_task` (
                                      `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                      `task_id` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '任务ID(UUID)',
                                      `upload_id` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL COMMENT '上传唯一ID',
                                      `parent_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL COMMENT '父ID',
                                      `user_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '用户ID',
                                      `workspace_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '工作空间ID',
                                      `storage_platform_setting_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL COMMENT '存储平台配置ID',
                                      `object_key` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '对象Key',
                                      `file_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL COMMENT '下载时关联的文件ID',
                                      `file_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '文件名',
                                      `file_size` bigint NOT NULL COMMENT '文件大小(字节)',
                                      `file_md5` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL COMMENT '文件MD5值',
                                      `suffix` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '文件扩展名',
                                      `mime_type` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '标准MIME类型',
                                      `total_chunks` int NOT NULL COMMENT '总分片数',
                                      `task_type` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL COMMENT '任务类型：upload/download',
                                      `uploaded_chunks` int NOT NULL DEFAULT 0 COMMENT '已上传分片数',
                                      `chunk_size` bigint NOT NULL DEFAULT 5242880 COMMENT '分片大小(默认5MB)',
                                      `uploaded_size` bigint NOT NULL DEFAULT 0 COMMENT '已上传大小(字节)',
                                      `status` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT 'uploading' COMMENT '状态',
                                      `error_msg` varchar(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL COMMENT '错误信息',
                                      `start_time` datetime NOT NULL COMMENT '开始时间',
                                      `complete_time` datetime DEFAULT NULL COMMENT '完成时间',
                                      `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                      `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                      PRIMARY KEY (`id`) USING BTREE,
                                      UNIQUE KEY `uk_task_id` (`task_id`) USING BTREE,
                                      KEY `idx_user_id` (`user_id`) USING BTREE,
                                      KEY `idx_workspace_id` (`workspace_id`) USING BTREE,
                                      KEY `idx_file_md5` (`file_md5`) USING BTREE,
                                      KEY `idx_status` (`status`) USING BTREE,
                                      KEY `idx_task_type` (`task_type`) USING BTREE,
                                      KEY `idx_create_time` (`created_at`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='传输任务表' ROW_FORMAT=DYNAMIC;

-- ----------------------------
-- Table structure for storage_platform
-- ----------------------------
DROP TABLE IF EXISTS `storage_platform`;
CREATE TABLE `storage_platform` (
                                    `id` int NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                    `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '存储平台名称',
                                    `identifier` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '存储平台标识符',
                                    `config_scheme` json NOT NULL COMMENT '存储平台配置描述schema',
                                    `icon` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '存储平台图标',
                                    `link` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '存储平台链接',
                                    `is_default` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否默认存储平台 0否 1是',
                                    `desc` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '存储平台描述',
                                    PRIMARY KEY (`id`) USING BTREE,
                                    UNIQUE KEY `uk_identifier` (`identifier`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='存储平台' ROW_FORMAT=DYNAMIC;

-- ----------------------------
-- Table structure for storage_settings
-- ----------------------------
DROP TABLE IF EXISTS `storage_settings`;
CREATE TABLE `storage_settings` (
                                    `id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '主键ID',
                                    `storage_setting_name` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '存储配置名称',
                                    `platform_identifier` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '存储平台标识符',
                                    `config_data` json NOT NULL COMMENT '存储配置',
                                    `enabled` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否启用 0否 1是',
                                    `workspace_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '所属工作空间',
                                    `created_at` datetime DEFAULT NULL COMMENT '创建时间',
                                    `updated_at` datetime DEFAULT NULL COMMENT '更新时间',
                                    `remark` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '备注',
                                    `deleted` tinyint(1) NOT NULL DEFAULT 0 COMMENT '逻辑删除 0未删除 1已删除',
                                    PRIMARY KEY (`id`) USING BTREE,
                                    KEY `idx_workspace_enabled` (`workspace_id`, `enabled`, `deleted`) USING BTREE,
                                    KEY `idx_platform_identifier` (`platform_identifier`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='存储平台配置' ROW_FORMAT=DYNAMIC;

-- ----------------------------
-- Table structure for user
-- ----------------------------
DROP TABLE IF EXISTS `user`;
CREATE TABLE `user` (
                        `id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '用户ID',
                        `username` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '用户名',
                        `password` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '密码',
                        `email` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '邮箱',
                        `nickname` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '昵称',
                        `avatar` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '头像',
                        `default_workspace_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '默认工作空间ID',
                        `status` int NOT NULL DEFAULT 0 COMMENT '用户状态 0正常 1禁用',
                        `created_at` datetime NOT NULL COMMENT '创建时间',
                        `updated_at` datetime NOT NULL COMMENT '更新时间',
                        `last_login_at` datetime DEFAULT NULL COMMENT '最后登录时间',
                        PRIMARY KEY (`id`) USING BTREE,
                        UNIQUE KEY `uk_username` (`username`) USING BTREE,
                        UNIQUE KEY `uk_email` (`email`) USING BTREE,
                        KEY `idx_default_workspace_id` (`default_workspace_id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户表' ROW_FORMAT=DYNAMIC;

-- ----------------------------
-- Seed data for user
-- ----------------------------
INSERT INTO `user` (
    `id`, `username`, `password`, `email`, `nickname`, `avatar`, `default_workspace_id`, `status`, `created_at`, `updated_at`, `last_login_at`
) VALUES (
             '01jrvgs943q0f43h0aa5mjde0y',
             'admin',
             '8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918',
             '111222333@qq.com',
             'lyjy',
             'https://csdn-665-inscode.s3.cn-north-1.jdcloud-oss.com/inscode/202303/628c9f991a7e4862742d8a2f/1680072908255-49035150-ttVQUH7YUEaCdHRZenaoQrUQPxtaBUay/large',
             'ws_01jrvgs943q0f43h0aa5mjde0y_personal',
             0,
             '2025-04-15 09:25:22',
             '2025-11-17 14:05:14',
             '2025-11-17 14:05:14'
         );

-- ----------------------------
-- Table structure for workspace
-- ----------------------------
DROP TABLE IF EXISTS `workspace`;
CREATE TABLE `workspace` (
                             `id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '工作空间ID',
                             `name` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '工作空间名称',
                             `workspace_type` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'personal/org',
                             `owner_user_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '创建者用户ID',
                             `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '状态 1启用 0禁用',
                             `created_at` datetime NOT NULL COMMENT '创建时间',
                             `updated_at` datetime NOT NULL COMMENT '更新时间',
                             PRIMARY KEY (`id`) USING BTREE,
                             KEY `idx_owner_user_id` (`owner_user_id`) USING BTREE,
                             KEY `idx_workspace_type` (`workspace_type`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='工作空间表' ROW_FORMAT=DYNAMIC;

-- ----------------------------
-- Records of workspace
-- ----------------------------
INSERT INTO `workspace` (
    `id`, `name`, `workspace_type`, `owner_user_id`, `status`, `created_at`, `updated_at`
) VALUES (
             'ws_01jrvgs943q0f43h0aa5mjde0y_personal',
             'admin 个人空间',
             'personal',
             '01jrvgs943q0f43h0aa5mjde0y',
             1,
             '2025-04-15 09:25:22',
             '2025-11-17 14:05:14'
         );

-- ----------------------------
-- Table structure for workspace_member
-- ----------------------------
DROP TABLE IF EXISTS `workspace_member`;
CREATE TABLE `workspace_member` (
                                    `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                    `workspace_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '工作空间ID',
                                    `user_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '用户ID',
                                    `role` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '角色 owner/admin/member',
                                    `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '状态 1正常 0移除',
                                    `joined_at` datetime NOT NULL COMMENT '加入时间',
                                    `created_at` datetime NOT NULL COMMENT '创建时间',
                                    `updated_at` datetime NOT NULL COMMENT '更新时间',
                                    PRIMARY KEY (`id`) USING BTREE,
                                    UNIQUE KEY `uk_workspace_user` (`workspace_id`, `user_id`) USING BTREE,
                                    KEY `idx_user_id` (`user_id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='工作空间成员表' ROW_FORMAT=DYNAMIC;

-- ----------------------------
-- Records of workspace_member
-- ----------------------------
INSERT INTO `workspace_member` (
    `id`, `workspace_id`, `user_id`, `role`, `status`, `joined_at`, `created_at`, `updated_at`
) VALUES (
             1,
             'ws_01jrvgs943q0f43h0aa5mjde0y_personal',
             '01jrvgs943q0f43h0aa5mjde0y',
             'owner',
             1,
             '2025-04-15 09:25:22',
             '2025-04-15 09:25:22',
             '2025-11-17 14:05:14'
         );

-- ----------------------------
-- Table structure for sys_user_transfer_setting
-- ----------------------------
DROP TABLE IF EXISTS `user_transfer_setting`;
CREATE TABLE `user_transfer_setting` (
                                             `id` bigint NOT NULL AUTO_INCREMENT,
                                             `user_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '用户ID',
                                             `download_location` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '文件下载位置',
                                             `is_default_download_location` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否默认下载路径 0否 1是',
                                             `download_speed_limit` int NOT NULL DEFAULT 5 COMMENT '下载速率限制，单位MB/S',
                                             `concurrent_upload_quantity` int NOT NULL DEFAULT 1 COMMENT '并发上传数量',
                                             `concurrent_download_quantity` int NOT NULL DEFAULT 1 COMMENT '并发下载数量',
                                             `chunk_size` bigint NOT NULL DEFAULT 5242880 COMMENT '分片大小，单位字节，默认5MB',
                                             `created_at` datetime NOT NULL COMMENT '创建时间',
                                             `updated_at` datetime NOT NULL COMMENT '修改时间',
                                             PRIMARY KEY (`id`) USING BTREE,
                                             UNIQUE KEY `uk_user_id` (`user_id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户传输设置' ROW_FORMAT=DYNAMIC;

-- ----------------------------
-- Records of sys_user_transfer_setting
-- ----------------------------
INSERT INTO `user_transfer_setting` (
    `id`, `user_id`, `download_location`, `is_default_download_location`, `download_speed_limit`,
    `concurrent_upload_quantity`, `concurrent_download_quantity`, `chunk_size`, `created_at`, `updated_at`
) VALUES (
             1,
             '01jrvgs943q0f43h0aa5mjde0y',
             NULL,
             0,
             5,
             1,
             1,
             5242880,
             '2025-11-11 14:45:27',
             '2025-11-11 14:45:29'
         );

SET FOREIGN_KEY_CHECKS = 1;

-- ============================================================
-- Agent 模块表
-- ============================================================

-- ----------------------------
-- Table structure for agent_action
-- ----------------------------
DROP TABLE IF EXISTS `agent_action`;
CREATE TABLE `agent_action` (
    `id` VARCHAR(128) NOT NULL COMMENT '主键ID',
    `session_id` VARCHAR(128) DEFAULT NULL COMMENT '会话ID',
    `workspace_id` VARCHAR(128) NOT NULL COMMENT '工作空间ID',
    `user_id` VARCHAR(128) NOT NULL COMMENT '用户ID',
    `user_input` TEXT COMMENT '用户输入',
    `run_type` VARCHAR(32) NOT NULL COMMENT '类型：retrieve / execute / rag / workflow',
    `status` VARCHAR(32) NOT NULL COMMENT '状态：running / success / failed / waiting_confirm',
    `risk_level` VARCHAR(32) DEFAULT 'read' COMMENT '风险等级：read / write / danger / export / cross_ws',
    `is_confirm` VARCHAR(128) DEFAULT NULL COMMENT '风险操作用户是否同意执行',
    `trace_id` VARCHAR(64) DEFAULT NULL COMMENT '链路追踪ID',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    INDEX `idx_session` (`session_id`),
    INDEX `idx_workspace_user` (`workspace_id`, `user_id`),
    INDEX `idx_status` (`status`),
    INDEX `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Agent 执行记录';

-- ----------------------------
-- Table structure for agent_action_step
-- ----------------------------
DROP TABLE IF EXISTS `agent_action_step`;
CREATE TABLE `agent_action_step` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    `run_id` VARCHAR(128) NOT NULL COMMENT '所属执行ID',
    `step_no` INT NOT NULL COMMENT '步骤序号',
    `step_type` VARCHAR(32) NOT NULL COMMENT '类型：plan / tool_call / observe / final',
    `content` TEXT COMMENT '步骤内容（思考/总结/中间结果）',
    `status` VARCHAR(32) NOT NULL COMMENT '状态：running / success / failed',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    INDEX `idx_run` (`run_id`),
    INDEX `idx_run_step` (`run_id`, `step_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Agent 执行步骤';

-- ----------------------------
-- Table structure for agent_prompt_template
-- ----------------------------
DROP TABLE IF EXISTS `agent_prompt_template`;
CREATE TABLE `agent_prompt_template` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    `name` VARCHAR(128) NOT NULL COMMENT '模板名称',
    `version` INT NOT NULL DEFAULT 1 COMMENT '版本号',
    `content` TEXT NOT NULL COMMENT '模板内容',
    `enabled` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    UNIQUE KEY `uk_name_version` (`name`, `version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Agent 提示词模板';

-- ----------------------------
-- Table structure for agent_tool
-- ----------------------------
DROP TABLE IF EXISTS `agent_tool`;
CREATE TABLE `agent_tool` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    `name` VARCHAR(128) NOT NULL COMMENT '工具唯一名称，如 tool.file.search',
    `description` VARCHAR(512) DEFAULT NULL COMMENT '工具描述，给 LLM 使用',
    `schema_json` JSON COMMENT '工具输入输出 schema',
    `risk_level` VARCHAR(32) DEFAULT 'read' COMMENT '风险等级：read / write / danger / export / cross_ws',
    `enabled` TINYINT(1) DEFAULT 1 COMMENT '是否启用',
    `timeout_ms` INT DEFAULT 5000 COMMENT '超时时间',
    `retry_times` INT DEFAULT 0 COMMENT '重试次数',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Agent 工具注册表';

-- ----------------------------
-- Table structure for agent_tool_call
-- ----------------------------
DROP TABLE IF EXISTS `agent_tool_call`;
CREATE TABLE `agent_tool_call` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    `run_id` VARCHAR(128) NOT NULL COMMENT '所属执行ID',
    `step_id` BIGINT NOT NULL COMMENT '所属步骤ID',
    `tool_name` VARCHAR(128) NOT NULL COMMENT '工具名',
    `input_json` JSON COMMENT '工具输入参数',
    `output_json` JSON COMMENT '工具输出结果',
    `status` VARCHAR(32) NOT NULL COMMENT '状态：success / failed',
    `error_message` TEXT COMMENT '错误信息',
    `idempotency_key` VARCHAR(128) DEFAULT NULL COMMENT '幂等key',
    `cost_ms` INT DEFAULT NULL COMMENT '耗时毫秒',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    INDEX `idx_run` (`run_id`),
    INDEX `idx_step` (`step_id`),
    INDEX `idx_tool` (`tool_name`),
    INDEX `idx_idempotency` (`idempotency_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='工具调用记录';

-- ============================================================
-- RAG 知识库模块
-- ============================================================

-- ----------------------------
-- Table structure for knowledge
-- ----------------------------
DROP TABLE IF EXISTS `knowledge`;
CREATE TABLE `knowledge` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    `workspace_id` VARCHAR(128) NOT NULL COMMENT '工作空间ID',
    `name` VARCHAR(128) NOT NULL COMMENT '知识库名称',
    `description` VARCHAR(512) DEFAULT NULL COMMENT '知识库描述',
    `created_by` VARCHAR(128) NOT NULL COMMENT '创建者用户ID',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    INDEX `idx_workspace` (`workspace_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='知识库';

-- ----------------------------
-- Table structure for knowledge_file
-- ----------------------------
DROP TABLE IF EXISTS `knowledge_file`;
CREATE TABLE `knowledge_file` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    `knowledge_base_id` BIGINT NOT NULL COMMENT '所属知识库ID',
    `workspace_id` VARCHAR(128) NOT NULL COMMENT '工作空间ID',
    `storage_setting_id` VARCHAR(128) NOT NULL COMMENT '存储配置ID',
    `file_id` VARCHAR(128) NOT NULL COMMENT '文件ID',
    `parse_status` VARCHAR(32) DEFAULT 'pending' COMMENT '解析状态',
    `chunk_status` VARCHAR(32) DEFAULT 'pending' COMMENT '切片状态',
    `embed_status` VARCHAR(32) DEFAULT 'pending' COMMENT '向量化状态',
    `index_status` VARCHAR(32) DEFAULT 'pending' COMMENT '索引状态',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    INDEX `idx_kb` (`knowledge_base_id`),
    INDEX `idx_workspace` (`workspace_id`),
    INDEX `idx_file` (`file_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='知识库文件';

-- ----------------------------
-- Table structure for knowledge_import_task
-- ----------------------------
DROP TABLE IF EXISTS `knowledge_import_task`;
CREATE TABLE `knowledge_import_task` (
    `id` VARCHAR(64) NOT NULL COMMENT '任务ID',
    `workspace_id` VARCHAR(128) NOT NULL COMMENT '工作空间ID',
    `knowledge_base_id` BIGINT NOT NULL COMMENT '知识库ID',
    `knowledge_file_id` BIGINT NOT NULL COMMENT '知识库文件记录ID',
    `file_id` VARCHAR(128) NOT NULL COMMENT '文件ID',
    `storage_setting_id` VARCHAR(128) NOT NULL COMMENT '存储配置ID',
    `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/running/success/failed',
    `stage` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/parsing/chunking/embedding/indexing/done',
    `progress` INT NOT NULL DEFAULT 0 COMMENT '0-100',
    `error_category` VARCHAR(32) DEFAULT NULL COMMENT '错误分类',
    `error_message` TEXT DEFAULT NULL COMMENT '错误信息',
    `retry_count` INT NOT NULL DEFAULT 0 COMMENT '重试次数',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    INDEX `idx_kb_file` (`workspace_id`, `knowledge_base_id`, `file_id`),
    INDEX `idx_status_stage` (`status`, `stage`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='知识库导入任务';

-- ----------------------------
-- Table structure for knowledge_document_chunk
-- ----------------------------
DROP TABLE IF EXISTS `knowledge_document_chunk`;
CREATE TABLE `knowledge_document_chunk` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    `knowledge_base_id` BIGINT NOT NULL COMMENT '所属知识库ID',
    `file_id` VARCHAR(128) NOT NULL COMMENT '文件ID',
    `chunk_no` INT NOT NULL COMMENT '切片序号',
    `content` TEXT COMMENT '切片内容',
    `token_count` INT DEFAULT NULL COMMENT 'Token 数',
    `vector_id` VARCHAR(128) DEFAULT NULL COMMENT '向量库中的ID',
    `metadata_json` JSON COMMENT '元数据（含 workspaceId / storageSettingId 等）',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    INDEX `idx_kb` (`knowledge_base_id`),
    INDEX `idx_file` (`file_id`),
    INDEX `idx_kb_file` (`knowledge_base_id`, `file_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='文档切片';

-- ============================================================
-- Workflow 模块
-- ============================================================

-- ----------------------------
-- Table structure for workflow_definition
-- ----------------------------
DROP TABLE IF EXISTS `workflow_definition`;
CREATE TABLE `workflow_definition` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    `workspace_id` VARCHAR(128) NOT NULL COMMENT '工作空间ID',
    `name` VARCHAR(128) NOT NULL COMMENT '工作流名称',
    `description` VARCHAR(512) DEFAULT NULL COMMENT '工作流描述',
    `dsl_json` JSON COMMENT 'Workflow 定义 JSON',
    `enabled` TINYINT(1) DEFAULT 1 COMMENT '是否启用',
    `created_by` VARCHAR(128) NOT NULL COMMENT '创建者用户ID',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    INDEX `idx_workspace` (`workspace_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='工作流定义';

-- ----------------------------
-- Table structure for workflow_run
-- ----------------------------
DROP TABLE IF EXISTS `workflow_run`;
CREATE TABLE `workflow_run` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    `workflow_id` BIGINT NOT NULL COMMENT '所属工作流定义ID',
    `workspace_id` VARCHAR(128) NOT NULL COMMENT '工作空间ID',
    `user_id` VARCHAR(128) NOT NULL COMMENT '用户ID',
    `input_json` JSON COMMENT '输入参数',
    `status` VARCHAR(32) NOT NULL COMMENT '状态：running / success / failed',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    INDEX `idx_workflow` (`workflow_id`),
    INDEX `idx_workspace_user` (`workspace_id`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='工作流运行';

-- ----------------------------
-- Table structure for workflow_node_run
-- ----------------------------
DROP TABLE IF EXISTS `workflow_node_run`;
CREATE TABLE `workflow_node_run` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    `workflow_run_id` BIGINT NOT NULL COMMENT '所属工作流运行ID',
    `node_id` VARCHAR(64) NOT NULL COMMENT '节点ID',
    `node_type` VARCHAR(32) NOT NULL COMMENT '节点类型',
    `input_json` JSON COMMENT '输入参数',
    `output_json` JSON COMMENT '输出结果',
    `status` VARCHAR(32) NOT NULL COMMENT '状态',
    `error_message` TEXT COMMENT '错误信息',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    INDEX `idx_run` (`workflow_run_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='工作流节点运行';
