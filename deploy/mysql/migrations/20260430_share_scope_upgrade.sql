-- 2026-04-30: share_info 增加工作空间和存储配置归属字段，并回填历史数据。

ALTER TABLE `share_info`
  ADD COLUMN `workspace_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL COMMENT '归属工作空间ID' AFTER `user_id`,
  ADD COLUMN `storage_setting_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL COMMENT '归属存储配置ID' AFTER `workspace_id`;

-- 通过 share_items -> file_info 回填历史分享归属（取最早一条关联文件作为归属来源）。
UPDATE `share_info` s
JOIN (
  SELECT si.share_id,
         SUBSTRING_INDEX(GROUP_CONCAT(COALESCE(fi.workspace_id, '') ORDER BY si.created_at ASC SEPARATOR ','), ',', 1) AS workspace_id,
         SUBSTRING_INDEX(GROUP_CONCAT(COALESCE(fi.storage_platform_setting_id, '') ORDER BY si.created_at ASC SEPARATOR ','), ',', 1) AS storage_setting_id
  FROM `share_items` si
  JOIN `file_info` fi ON fi.id = si.file_id
  GROUP BY si.share_id
) x ON x.share_id = s.id
SET s.workspace_id = NULLIF(x.workspace_id, ''),
    s.storage_setting_id = NULLIF(x.storage_setting_id, '');

-- 兜底：按 user 默认工作空间补齐，存储配置缺失则空串。
UPDATE `share_info` s
JOIN `user` u ON u.id = s.user_id
SET s.workspace_id = COALESCE(s.workspace_id, u.default_workspace_id),
    s.storage_setting_id = COALESCE(s.storage_setting_id, '')
WHERE s.workspace_id IS NULL OR s.storage_setting_id IS NULL;

ALTER TABLE `share_info`
  MODIFY COLUMN `workspace_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '归属工作空间ID',
  MODIFY COLUMN `storage_setting_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '归属存储配置ID';

ALTER TABLE `share_info`
  ADD KEY `idx_workspace_id` (`workspace_id`) USING BTREE,
  ADD KEY `idx_workspace_setting` (`workspace_id`, `storage_setting_id`) USING BTREE;
