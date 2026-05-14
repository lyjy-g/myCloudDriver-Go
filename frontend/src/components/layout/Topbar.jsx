import { Button, Space, Tag, Typography } from "antd";
import { EnvironmentOutlined, IdcardOutlined, SwapOutlined, UserOutlined } from "@ant-design/icons";
import React from "react";

const { Text } = Typography;

function maskPath(path) {
  if (!path) {
    return "-";
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
 * 顶部栏。
 *
 * @param {{
 *   topPromos: Array<{key: string, label: string, tone: string}>,
 *   activeWorkspace: object,
 *   activeStorage: object,
 *   currentUser: object,
 *   onOpenStorageSettings: Function
 * }} props 组件参数
 * @returns {JSX.Element} 顶部栏
 */
export function Topbar({ topPromos, activeWorkspace, activeStorage, currentUser, onOpenStorageSettings }) {
  const storageLabel = activeStorage?.name || activeStorage?.identifier || "未配置";
  const storagePath = maskPath(activeStorage?.basePath);
  const storageSettingId = activeStorage?.settingId || "-";
  const workspaceLabel = activeWorkspace?.workspaceName || activeWorkspace?.workspaceId || "未选择空间";
  const userLabel = currentUser?.displayName || currentUser?.username || "匿名";

  return (
    <header className="mcd-topbar">
      <div className="mcd-topbar-inner">
        <div className="mcd-topbar-brand">
          <div className="mcd-avatar">M</div>
          <div className="mcd-topbar-brand-text">
            <div className="mcd-topbar-brand-title">MyCloudDrive</div>
            <div className="mcd-topbar-brand-sub">云盘与知识库协同工作台</div>
          </div>
          <div className="mcd-topbar-promos">
            {topPromos.map((promo) => (
              <div
                key={promo.key}
                className={
                  promo.tone === "warm"
                    ? "mcd-pill mcd-pill-warm"
                    : promo.tone === "outline"
                    ? "mcd-pill mcd-pill-outline"
                    : "mcd-pill"
                }
              >
                {promo.label}
              </div>
            ))}
          </div>
        </div>
        <div className="mcd-topbar-meta">
          <div className="mcd-topbar-meta-item">
            <EnvironmentOutlined />
            <span>{workspaceLabel}</span>
          </div>
          <div className="mcd-topbar-meta-item">
            <IdcardOutlined />
            <span>{storageLabel}</span>
          </div>
          <div className="mcd-topbar-meta-item mcd-topbar-meta-item-sub">
            <SwapOutlined />
            <span>{storagePath}</span>
          </div>
          <div className="mcd-topbar-meta-item">
            <UserOutlined />
            <span>{userLabel}</span>
          </div>
          <Button onClick={() => onOpenStorageSettings?.("workspace", storageSettingId)}>
            工作区设置
          </Button>
        </div>
      </div>
    </header>
  );
}
