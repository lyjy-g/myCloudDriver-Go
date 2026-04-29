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
        <div className="flex items-center gap-3">
          <div className="mcd-avatar">M</div>
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
    </header>
  );
}
