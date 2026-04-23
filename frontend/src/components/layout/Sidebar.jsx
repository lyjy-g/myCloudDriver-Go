import { Button, Menu } from "antd";
import { ApartmentOutlined, DatabaseOutlined, SettingOutlined } from "@ant-design/icons";
import React from "react";

/**
 * 左侧菜单栏。
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
  menuItems,
  typeFilters,
  workspaces,
  activeWorkspace,
  activeStorage,
  onSwitchWorkspace,
  onOpenStorageSettings,
  onRefreshWorkspace
}) {
  const workspaceItems = (workspaces || []).map((item) => ({
    key: `ws:${item.workspaceId}`,
    label: `${item.workspaceName}（${item.workspaceType}）`
  }));

  const workspaceMenuItems = [
    {
      key: "workspace-home",
      icon: <ApartmentOutlined />,
      label: "我的空间",
      children: workspaceItems
    },
    {
      key: "workspace-settings",
      icon: <SettingOutlined />,
      label: "空间配置"
    },
    {
      key: "workspace-storage",
      icon: <DatabaseOutlined />,
      label: activeStorage?.identifier || "未配置存储",
      disabled: true
    }
  ];

  const selectedWorkspaceKeys = (() => {
    if (activeMenu === "settings" || activeMenu === "workspace-settings") {
      return ["workspace-settings"];
    }
    if (activeMenu === "workspace-home") {
      return ["workspace-home"];
    }
    if (activeWorkspace?.workspaceId) {
      return [`ws:${activeWorkspace.workspaceId}`];
    }
    return [];
  })();

  return (
    <aside className="mcd-sidebar">
      <div className="mcd-sidebar-card">
        <Button type="link" className="mcd-sidebar-title">
          工作空间
        </Button>
      </div>

      <Menu
        mode="inline"
        selectedKeys={selectedWorkspaceKeys}
        defaultOpenKeys={["workspace-home"]}
        onClick={({ key }) => {
          if (key === "workspace-home") {
            onRefreshWorkspace?.();
            onMenuClick("workspace-home");
            return;
          }
          if (key === "workspace-settings") {
            onMenuClick("workspace-settings");
            onOpenStorageSettings?.();
            return;
          }
          if (typeof key === "string" && key.startsWith("ws:")) {
            const workspaceId = key.slice(3);
            onSwitchWorkspace?.(workspaceId);
            onMenuClick("workspace-home");
          }
        }}
        items={workspaceMenuItems}
      />

      <div className="mcd-sidebar-card">
        <Button type="link" className="mcd-sidebar-title">
          我的文件
        </Button>
      </div>
      <Menu
        mode="inline"
        selectedKeys={[activeMenu]}
        onClick={({ key }) => onMenuClick(key)}
        items={menuItems.map((item) => {
          const MenuIcon = item.icon;
          return {
            ...item,
            icon: MenuIcon ? <MenuIcon /> : null
          };
        })}
      />
      <div className="mcd-sidebar-section">
        <span className="mcd-muted">分类</span>
        <div className="mcd-sidebar-list">
          {typeFilters.map((item) => {
            const FilterIcon = item.icon;
            return (
              <button key={item.key} type="button" className="mcd-sidebar-item">
                {FilterIcon ? <FilterIcon /> : null}
                <span>{item.label}</span>
              </button>
            );
          })}
        </div>
      </div>
    </aside>
  );
}
