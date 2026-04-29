import { App as AntApp, Button, Card, Input, Space, Typography } from "antd";
import React, { useState } from "react";
import {
  ICON_RAIL_ITEMS,
  LOCAL_PATH_PRESETS,
  QUICK_ACTIONS,
  TOP_PROMOS
} from "./constants/appConfig.js";
import { Toolbar } from "./components/files/Toolbar.jsx";
import { FilesPanel } from "./components/files/FilesPanel.jsx";
import { IconRail } from "./components/layout/IconRail.jsx";
import { Sidebar } from "./components/layout/Sidebar.jsx";
import { Topbar } from "./components/layout/Topbar.jsx";
import { StorageSettingsPanel } from "./components/settings/StorageSettingsPanel.jsx";
import { SharePublicPanel } from "./components/share/SharePublicPanel.jsx";
import { UploadDrawerPanel } from "./components/upload/UploadDrawerPanel.jsx";
import { useBaseUrl } from "./hooks/useBaseUrl.js";
import { useCloudDriveController } from "./hooks/useCloudDriveController.jsx";
import { useNotifier } from "./hooks/useNotifier.js";

/**
 * 网盘主界面。
 *
 * @returns {JSX.Element} 页面
 */
export default function App() {
  const [loginForm, setLoginForm] = useState({ username: "admin", password: "admin" });
  const [storageSettingsOpen, setStorageSettingsOpen] = useState(false);
  const [settingsView, setSettingsView] = useState("workspace");
  const { contextHolder, notifySuccess, notifyWarning, notifyError } = useNotifier();
  const { normalizedBaseUrl } = useBaseUrl();

  const controller = useCloudDriveController(normalizedBaseUrl, {
    notifySuccess,
    notifyWarning,
    notifyError
  });
  const openStorageSettings = (view = "workspace", settingId = "") => {
    setSettingsView(view);
    if (view === "editor" && settingId) {
      controller.handleEditStorageSetting(settingId);
    }
    controller.setActiveMenu("workspace-home");
    setStorageSettingsOpen(true);
  };

  const handleLoginSubmit = async () => {
    await controller.handleLogin(loginForm.username, loginForm.password);
  };

  const sharePageMatch = window.location.pathname.match(/^\/share\/([A-Za-z0-9_-]+)$/);
  if (sharePageMatch) {
    return (
      <AntApp>
        {contextHolder}
        <SharePublicPanel normalizedBaseUrl={normalizedBaseUrl} shareId={sharePageMatch[1]} />
      </AntApp>
    );
  }

  if (!controller.authenticated) {
    return (
      <AntApp>
        {contextHolder}
        <div className="mcd-login-shell">
          <div className="mcd-login-glow" />
          <Card title="MyCloudDrive 登录" className="mcd-login-card">
            <Space direction="vertical" style={{ width: "100%" }} size="middle">
              <Typography.Text type="secondary">个人云盘管理入口，支持工作空间与团队协作模式。</Typography.Text>
              <Input
                placeholder="用户名"
                value={loginForm.username}
                onChange={(event) => setLoginForm((prev) => ({ ...prev, username: event.target.value }))}
              />
              <Input.Password
                placeholder="密码"
                value={loginForm.password}
                onChange={(event) => setLoginForm((prev) => ({ ...prev, password: event.target.value }))}
                onPressEnter={handleLoginSubmit}
              />
              <Button type="primary" className="mcd-primary-btn" onClick={handleLoginSubmit} loading={controller.loading}>
                登录并进入控制台
              </Button>
            </Space>
          </Card>
        </div>
      </AntApp>
    );
  }

  return (
    <AntApp>
      {contextHolder}
      <div className="mcd-shell">
        <Topbar
          topPromos={TOP_PROMOS}
          activeWorkspace={controller.activeWorkspace}
          activeStorage={controller.activeStorage}
          currentUser={controller.currentUser}
          onOpenStorageSettings={openStorageSettings}
        />

        <div className="mcd-body">
          <Sidebar
            activeMenu={controller.activeMenu}
            onMenuClick={controller.setActiveMenu}
            workspaces={controller.workspaces}
            activeWorkspace={controller.activeWorkspace}
            activeStorage={controller.activeStorage}
            storageSettings={controller.storageSettings}
            enabledStorageIds={controller.enabledStorageIds}
            onSwitchWorkspace={controller.handleSwitchWorkspace}
            onOpenStorageSettings={openStorageSettings}
            onActivateStorageSetting={controller.handleActivateStorageSetting}
            onEnableStorageSetting={controller.handleEnableStorageSetting}
          />

          <main className="mcd-main">
            {["files", "shares", "trash"].includes(controller.activeMenu) ? (
              <Toolbar
                quickActions={QUICK_ACTIONS}
                activeMenu={controller.activeMenu}
                onLogout={controller.handleLogout}
                onUpload={() => controller.setDrawerOpen(true)}
                onCreateFolder={controller.handleCreateFolder}
                onRefreshFiles={controller.loadFiles}
                onRefreshShares={controller.loadShares}
                onRefreshTrash={controller.loadRecycleBin}
              />
            ) : null}

            <FilesPanel
              activeMenu={controller.activeMenu}
              files={controller.files}
              shares={controller.shares}
              columns={controller.columns}
              shareColumns={controller.shareColumns}
              directoryTrail={controller.directoryTrail}
              onJumpDirectory={controller.jumpToDirectory}
              activeWorkspace={controller.activeWorkspace}
              activeStorage={controller.activeStorage}
              currentUser={controller.currentUser}
              storageSettings={controller.storageSettings}
              enabledStorageIds={controller.enabledStorageIds}
              platforms={controller.platforms}
              onOpenStorageSettings={openStorageSettings}
              onOpenFiles={() => controller.setActiveMenu("files")}
              onCreateStorageSetting={() => {
                controller.handleCreateStorageDraft();
                openStorageSettings("editor");
              }}
              onRefreshWorkspace={controller.loadStorageMeta}
              onEditStorageSetting={(settingId) => openStorageSettings("editor", settingId)}
              onEnableStorageSetting={controller.handleEnableStorageSetting}
              onDisableStorageSetting={controller.handleDisableStorageSetting}
              onDeleteStorageSetting={controller.handleDeleteStorageSetting}
            />

            <StorageSettingsPanel
              visible={storageSettingsOpen}
              activeView={settingsView}
              workspaces={controller.workspaces}
              activeWorkspace={controller.activeWorkspace}
              platforms={controller.platforms}
              storageSettings={controller.storageSettings}
              storageForm={controller.storageForm}
              loading={controller.loading}
              normalizedBaseUrl={normalizedBaseUrl}
              localPathPresets={LOCAL_PATH_PRESETS}
              onClose={() => setStorageSettingsOpen(false)}
              onRefresh={controller.loadStorageMeta}
              onUpdateField={controller.updateStorageFormField}
              onSwitchWorkspace={controller.handleSwitchWorkspace}
              onApply={controller.handleApplyStorageConfig}
              onEditSetting={controller.handleEditStorageSetting}
              onCreateDraft={controller.handleCreateStorageDraft}
              onActivateSetting={controller.handleActivateStorageSetting}
              onRebuildIndexes={controller.handleRebuildIndexes}
              onCreateWorkspace={controller.handleCreateWorkspace}
              onRenameWorkspace={controller.handleRenameWorkspace}
              onDeleteWorkspace={controller.handleDeleteWorkspace}
              onAddWorkspaceUser={controller.handleAddWorkspaceUser}
              onRemoveWorkspaceUser={controller.handleRemoveWorkspaceUser}
              onDeleteSetting={controller.handleDeleteStorageSetting}
            />

            <div className="mcd-footer-chat">
              <div className="mcd-footer-bubble">知识搜索/问答/创作/AI全网搜索</div>
              <div className="mcd-footer-actions">
                <button type="button" className="mcd-footer-btn">↑</button>
                <button type="button" className="mcd-footer-btn primary">✦</button>
              </div>
            </div>
          </main>
        </div>

        <UploadDrawerPanel
          open={controller.drawerOpen}
          loading={controller.loading}
          uploadProgress={controller.uploadProgress}
          onClose={() => controller.setDrawerOpen(false)}
          onFileChange={(event) => controller.setSelectedFile(event.target.files?.[0] || null)}
          onUpload={controller.handleUpload}
        />
      </div>
    </AntApp>
  );
}
