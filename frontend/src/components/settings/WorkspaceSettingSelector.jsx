import { Button, Popconfirm, Space, Tag, Typography } from "antd";
import React from "react";

const { Title, Text } = Typography;

function maskPath(path) {
  if (!path) {
    return "(未设置)";
  }
  const normalized = String(path).replace(/\\/g, "/");
  if (!normalized.startsWith("/")) {
    return normalized;
  }
  const segments = normalized.split("/").filter(Boolean);
  if (segments.length <= 2) {
    return "/***";
  }
  return `/***/${segments.slice(-2).join("/")}`;
}

/**
 * 当前空间配置选择器。
 *
 * @param {{
 *   storageSettings: Array,
 *   enabledStorageIds: Array,
 *   onEditSetting: Function,
 *   onEnableSetting: Function,
 *   onDisableSetting: Function,
 *   onDeleteSetting: Function
 * }} props 组件参数
 * @returns {JSX.Element} 选择器
 */
export function WorkspaceSettingSelector({
  storageSettings,
  enabledStorageIds,
  onEditSetting,
  onEnableSetting,
  onDisableSetting,
  onDeleteSetting
}) {
  const enabledSet = new Set(enabledStorageIds || []);
  return (
    <div className="mcd-settings-selector">
      <div className="mcd-settings-quick-list">
        {(storageSettings || []).map((item) => (
          <div key={item.settingId} className={`mcd-settings-quick-item ${enabledSet.has(item.settingId) ? "active" : ""}`}>
            <div className="mcd-settings-quick-main">
              <div className="mcd-settings-quick-title">
                {item.name || item.identifier}
                <Tag color={enabledSet.has(item.settingId) ? "blue" : "default"} style={{ marginLeft: 8 }}>{enabledSet.has(item.settingId) ? "已启用" : "未启用"}</Tag>
              </div>
              <div className="mcd-settings-quick-sub">
                ID: {item.settingId || "-"} | 路径: {item.bucket || maskPath(item.basePath)}
              </div>
            </div>
            <Space>
              <Button size="small" onClick={() => onEditSetting(item.settingId)}>编辑</Button>
              {enabledSet.has(item.settingId) ? (
                <Button size="small" onClick={() => onDisableSetting(item.settingId)}>关闭</Button>
              ) : (
                <Button size="small" type="primary" ghost onClick={() => onEnableSetting(item.settingId)}>启用</Button>
              )}
              <Popconfirm
                title="确认删除该配置？"
                description="删除后不可恢复，请谨慎操作。"
                okText="确认删除"
                cancelText="取消"
                onConfirm={() => onDeleteSetting(item.settingId)}
              >
                <Button size="small" danger>删除</Button>
              </Popconfirm>
            </Space>
          </div>
        ))}
        {(storageSettings || []).length === 0 ? <Text className="mcd-muted">当前空间暂无存储配置</Text> : null}
      </div>
    </div>
  );
}
