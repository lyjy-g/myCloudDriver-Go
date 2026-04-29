import { ApartmentOutlined, DatabaseOutlined, SettingOutlined } from "@ant-design/icons";
import React from "react";

/**
 * 左侧菜单栏（重设计版）。
 *
 * @param {{
 *   activeMenu: string,
 *   onMenuClick: Function,
 *   menuItems: Array,
 *   typeFilters: Array,
 *   workspaces: Array,
 *   activeWorkspace: object,
 *   activeStorage: object,
 *   onSwitchWorkspace: Function,
 *   onOpenStorageSettings: Function,
 *   onRefreshWorkspace: Function
 * }} props 组件参数
 * @returns {JSX.Element} 侧栏
 */
export function Sidebar({
  activeMenu,
  onMenuClick,
  workspaces,
  activeWorkspace,
  activeStorage,
  storageSettings,
  enabledStorageIds,
  onSwitchWorkspace,
  onOpenStorageSettings,
  onActivateStorageSetting,
  onEnableStorageSetting
}) {
  const enabledSet = new Set(enabledStorageIds || []);
  const enabledSettings = (storageSettings || [])
    .filter((item) => enabledSet.has(item.settingId) || item.active)
    .sort((a, b) => {
      const aSelected = a.settingId === activeStorage?.settingId;
      const bSelected = b.settingId === activeStorage?.settingId;
      if (aSelected === bSelected) {
        return 0;
      }
      return aSelected ? -1 : 1;
    });
  const disabledSettings = (storageSettings || []).filter((item) => !enabledSet.has(item.settingId) && !item.active);

  return (
    <aside className="mcd-sidebar mcd-sidebar-redesign">
      <section className="mcd-sidebar-block">
        <div className="mcd-sidebar-l2-head">
          <ApartmentOutlined />
          <span>空间列表</span>
        </div>
        <div className="mcd-sidebar-workspaces">
          {(workspaces || []).map((item) => {
            const selected = item.workspaceId === activeWorkspace?.workspaceId;
            return (
              <button
                key={item.workspaceId}
                type="button"
                className={`mcd-sidebar-workspace-item ${selected ? "active" : ""}`}
                onClick={() => {
                  onSwitchWorkspace?.(item.workspaceId);
                  onMenuClick("workspace-home");
                }}
              >
                <span className="mcd-sidebar-workspace-name">{item.workspaceName}</span>
                <span className="mcd-sidebar-workspace-type">{item.workspaceType}</span>
              </button>
            );
          })}
        </div>
        <button
          type="button"
          className={`mcd-sidebar-nav-item ${activeMenu === "workspace-home" ? "active" : ""}`}
          onClick={() => {
            onMenuClick("workspace-home");
            onOpenStorageSettings?.("workspace");
          }}
        >
          <SettingOutlined />
          <span>空间配置</span>
        </button>
      </section>

      <section className="mcd-sidebar-block">
        <div className="mcd-sidebar-l2-head">
          <DatabaseOutlined />
          <span>配置列表</span>
        </div>
        <div className="mcd-sidebar-storage-list">
          {enabledSettings.map((item) => (
            <div key={item.settingId} className="mcd-sidebar-storage-item active">
              <div className="mcd-sidebar-storage-main">
                <div className="mcd-sidebar-storage-header">
                  <span className="mcd-sidebar-storage-name">{item.name || item.identifier}</span>
                  <span className="mcd-sidebar-storage-state active">
                    已启用
                  </span>
                </div>
                 </div>
              <div className="mcd-sidebar-storage-links">
                <div className="mcd-sidebar-storage-links-group">
                  {[
                    { key: "files", label: "全部文件" },
                    { key: "shares", label: "我的分享" },
                    { key: "trash", label: "回收站" }
                  ].map((entry) => (
                    <button
                      key={`${item.settingId}-${entry.key}`}
                      type="button"
                      className={`mcd-sidebar-nav-item mcd-sidebar-storage-link ${activeMenu === entry.key && activeStorage?.settingId === item.settingId ? "active" : ""}`}
                      onClick={async () => {
                        await onActivateStorageSetting?.(item.settingId);
                        onMenuClick(entry.key);
                      }}
                    >
                      {entry.label}
                    </button>
                  ))}
                </div>

              </div>
            </div>
          ))}
          {enabledSettings.length === 0 ? <span className="mcd-muted">暂无已启用配置</span> : null}

          <div className="mcd-sidebar-l3-head" style={{ marginTop: 12 }}>未启用配置</div>
          {disabledSettings.map((item) => (
            <div key={item.settingId} className="mcd-sidebar-storage-item">
              <div className="mcd-sidebar-storage-main">
                <div className="mcd-sidebar-storage-header">
                  <span className="mcd-sidebar-storage-name">{item.name || item.identifier}</span>
                  <button
                    type="button"
                    className="mcd-sidebar-storage-edit"
                    onClick={() => onEnableStorageSetting?.(item.settingId)}
                  >
                    启用
                  </button>
                </div>
              </div>
            </div>
          ))}
          {disabledSettings.length === 0 ? <span className="mcd-muted">暂无未启用配置</span> : null}
        </div>
      </section>
    </aside>
  );
}
