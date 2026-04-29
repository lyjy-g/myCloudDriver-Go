import { Button, Space, Tag, Typography } from "antd";
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
 *   onEditSetting: Function,
 *   onActivateSetting: Function
 * }} props 组件参数
 * @returns {JSX.Element} 选择器
 */
export function WorkspaceSettingSelector({
  storageSettings,
  onEditSetting,
  onActivateSetting
}) {
  return (
    <div className="mcd-settings-selector">
      <div className="mcd-settings-quick-list">
        {(storageSettings || []).map((item) => (
          <div key={item.settingId} className={`mcd-settings-quick-item ${item.active ? "active" : ""}`}>
            <div className="mcd-settings-quick-main">
              <div className="mcd-settings-quick-title">
                {item.name || item.identifier}
                <Tag color={item.active ? "blue" : "default"} style={{ marginLeft: 8 }}>{item.active ? "当前启用" : "未启用"}</Tag>
              </div>
              <div className="mcd-settings-quick-sub">
                ID: {item.settingId || "-"} | 路径: {item.bucket || maskPath(item.basePath)}
              </div>
            </div>
            <Space>
              <Button size="small" onClick={() => onEditSetting(item.settingId)}>编辑</Button>
              <Button size="small" type={item.active ? "default" : "primary"} ghost={!item.active} disabled={item.active} onClick={() => onActivateSetting(item.settingId)}>
                {item.active ? "已启用" : "启用"}
              </Button>
            </Space>
          </div>
        ))}
        {(storageSettings || []).length === 0 ? <Text className="mcd-muted">当前空间暂无存储配置</Text> : null}
      </div>
    </div>
  );
}
