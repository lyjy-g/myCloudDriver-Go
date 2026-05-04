import { App as AntApp, Button, Card, Input, Select, Space, Typography } from "antd";
import React, { useCallback, useEffect, useRef, useState } from "react";
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

// ============================================================
// 对话消息区域（可调整高度、load-more 顶部翻页、加载动效）
// ============================================================
function ChatArea({ messages, hasMore, loadingMore, onLoadMore, running }) {
  const containerRef = useRef(null);
  const [height, setHeight] = useState(320);
  const [resizing, setResizing] = useState(false);
  const [atTop, setAtTop] = useState(false);

  // 检查是否在顶部
  const checkScrollTop = useCallback(() => {
    const el = containerRef.current;
    if (!el) return;
    setAtTop(el.scrollTop <= 20);
  }, []);

  // 新消息或 streaming 时自动滚到底部
  const scrollToBottom = useCallback(() => {
    const el = containerRef.current;
    if (el) {
      requestAnimationFrame(() => { el.scrollTop = el.scrollHeight; });
    }
  }, []);

  useEffect(() => {
    if (running) scrollToBottom();
  }, [messages, running, scrollToBottom]);

  // 在 messages 增加且最后一条是 assistant streaming 时也滚到底部
  useEffect(() => {
    const msgs = messages;
    if (msgs.length > 0) {
      const last = msgs[msgs.length - 1];
      if (last.role === "assistant" && last.isStreaming) scrollToBottom();
    }
  }, [messages, scrollToBottom]);

  // 拖拽 resize
  const handleMouseDown = useCallback((e) => {
    e.preventDefault();
    setResizing(true);
    const startY = e.clientY;
    const startH = height;

    const onMove = (ev) => {
      const delta = startY - ev.clientY;
      const newH = Math.min(Math.max(startH + delta, 140), window.innerHeight * 0.55);
      setHeight(newH);
    };
    const onUp = () => {
      setResizing(false);
      window.removeEventListener("mousemove", onMove);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp, { once: true });
  }, [height]);

  if (messages.length === 0 && !running) {
    return (
      <div className="mcd-agent-chat-area" style={{ height }}>
        <div className="mcd-agent-resize-handle" onMouseDown={handleMouseDown} />
        <div className="mcd-agent-messages">
          <div className="mcd-agent-empty">等待你的问题，我会在这里展示回答结果。</div>
        </div>
      </div>
    );
  }

  return (
    <div className="mcd-agent-chat-area" style={{ height }}>
      <div className="mcd-agent-resize-handle" onMouseDown={handleMouseDown} />
      <div
        className="mcd-agent-messages"
        ref={containerRef}
        onScroll={checkScrollTop}
      >
        {hasMore && messages.length > 0 && messages[0].traceId ? (
          <div className="mcd-agent-load-more-wrap">
            {loadingMore ? (
              <span className="mcd-agent-loading-dots">加载中...</span>
            ) : (
              <button
                type="button"
                className="mcd-agent-load-more-btn"
                onClick={onLoadMore}
              >
                ↑ 查看更早的对话
              </button>
            )}
          </div>
        ) : !hasMore && messages.length > 0 ? (
          <div className="mcd-agent-history-end">—— 没有更早的记录了 ——</div>
        ) : null}

        {messages.map((msg) => (
          <div
            key={msg.id}
            className={`mcd-agent-msg mcd-agent-msg-${msg.role}`}
          >
            <div className="mcd-agent-msg-label">
              {msg.role === "user" ? "你" : "AI"}
            </div>
            <div className="mcd-agent-msg-content">
              {msg.role === "assistant" && msg.isStreaming && !msg.content ? (
                <span className="mcd-agent-thinking">
                  <span className="mcd-agent-dot-pulse">
                    <span style={{ animationDelay: "0s" }} />
                    <span style={{ animationDelay: ".2s" }} />
                    <span style={{ animationDelay: ".4s" }} />
                  </span>
                </span>
              ) : (
                msg.content || (msg.role === "assistant" ? "（空）" : "")
              )}
              {msg.role === "assistant" && msg.isStreaming && msg.content ? (
                <span className="mcd-agent-cursor-blink">|</span>
              ) : null}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

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
            knowledgeBases={controller.knowledgeBases}
            activeKnowledgeId={controller.activeKnowledgeId}
            onOpenKnowledgeHome={controller.handleOpenKnowledgeHome}
            onOpenKnowledgeBase={controller.handleOpenKnowledgeBase}
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
              knowledgeBases={controller.knowledgeBases}
              knowledgeFiles={controller.knowledgeFiles}
              activeKnowledge={controller.activeKnowledge}
              columns={controller.columns}
              shareColumns={controller.shareColumns}
              directoryTrail={controller.directoryTrail}
              onJumpDirectory={controller.jumpToDirectory}
              onOpenDirectory={controller.openDirectory}
              activeWorkspace={controller.activeWorkspace}
              activeStorage={controller.activeStorage}
              currentUser={controller.currentUser}
              storageSettings={controller.storageSettings}
              enabledStorageIds={controller.enabledStorageIds}
              platforms={controller.platforms}
              onOpenStorageSettings={openStorageSettings}
              onOpenFiles={() => controller.setActiveMenu("files")}
              onCreateStorageSetting={(draft) => {
                controller.handleCreateStorageDraft(draft);
                openStorageSettings("editor");
              }}
              onRefreshWorkspace={controller.loadStorageMeta}
              onEditStorageSetting={(settingId) => openStorageSettings("editor", settingId)}
              onEnableStorageSetting={controller.handleEnableStorageSetting}
              onDisableStorageSetting={controller.handleDisableStorageSetting}
              onDeleteStorageSetting={controller.handleDeleteStorageSetting}
              onCreateKnowledgeBase={controller.handleCreateKnowledgeBase}
              onDeleteKnowledgeBase={controller.handleDeleteKnowledgeBase}
              onAddKnowledgeFile={controller.handleAddKnowledgeFile}
              onAddKnowledgeItems={controller.handleAddKnowledgeItems}
              onSwitchKnowledgeImportStorage={controller.handleSwitchKnowledgeImportStorage}
              agentQuery={controller.agentQuery}
              agentResult={controller.agentResult}
              onAgentQueryChange={controller.setAgentQuery}
              onAgentSubmit={controller.handleAgentQuery}
              agentChatCollapsed={controller.agentChatCollapsed}
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

            <div className={`mcd-agent-float ${controller.agentInputCollapsed ? "collapsed" : "expanded"}`}>
              {!controller.agentInputCollapsed ? (
                <div className="mcd-agent-panel">
                  {!controller.agentChatCollapsed ? (
                    <ChatArea
                      messages={controller.agentMessages}
                      hasMore={controller.agentHistoryHasMore}
                      loadingMore={controller.agentLoadingHistory}
                      onLoadMore={() => {
                        // 找到最早有 traceId 的消息来翻页
                        const msgs = controller.agentMessages;
                        let cursor = "";
                        for (const m of msgs) {
                          if (m.traceId && m.traceId.startsWith("agt_")) {
                            cursor = m.traceId;
                            break;
                          }
                        }
                        if (cursor) controller.loadAgentHistory(cursor);
                      }}
                      running={controller.agentRunning}
                    />
                  ) : null}

                  <div className="mcd-agent-composer">
                    <Select
                      className="mcd-agent-select"
                      style={{ width: 110 }}
                      value={controller.agentScope}
                      onChange={controller.setAgentScope}
                      options={controller.agentScopeOptions}
                    />
                    <Select
                      className="mcd-agent-select"
                      style={{ width: 110 }}
                      value={controller.agentMode}
                      onChange={controller.setAgentMode}
                      options={[
                        { value: "search", label: "检索" },
                        { value: "rag", label: "RAG" },
                        { value: "workflow", label: "workflow" },
                        { value: "execute", label: "执行" }
                      ]}
                    />
                    <Select
                      className="mcd-agent-select"
                      style={{ width: 220 }}
                      value={controller.agentPresetQuestion || undefined}
                      placeholder="预设问题"
                      onChange={(value) => {
                        controller.setAgentPresetQuestion(value);
                        controller.setAgentQuery(value);
                      }}
                      options={controller.agentPresetOptions}
                      showSearch
                      optionFilterProp="label"
                    />
                    <Input
                      className="mcd-agent-input"
                      placeholder="知识搜索/问答/创作/AI全网搜索~"
                      value={controller.agentQuery}
                      onChange={(event) => controller.setAgentQuery(event.target.value)}
                      onPressEnter={controller.handleAgentQuery}
                      disabled={controller.agentRunning}
                    />
                    {controller.agentRunning ? (
                      <button
                        type="button"
                        className="mcd-agent-stop-btn"
                        onClick={controller.handleStopAgent}
                        title="停止"
                      >
                        <span className="mcd-agent-stop-icon">■</span>
                      </button>
                    ) : (
                      <button
                        type="button"
                        className="mcd-agent-send-btn"
                        onClick={controller.handleAgentQuery}
                      >
                        ➤
                      </button>
                    )}
                    <button
                      type="button"
                      className="mcd-agent-icon-btn ghost"
                      title={controller.agentChatCollapsed ? "展开对话" : "收起对话"}
                      onClick={() => controller.setAgentChatCollapsed(!controller.agentChatCollapsed)}
                      disabled={controller.agentRunning}
                    >
                      {controller.agentChatCollapsed ? "▾" : "▴"}
                    </button>
                    <button
                      type="button"
                      className="mcd-agent-icon-btn ghost"
                      title="关闭询问框"
                      onClick={() => controller.setAgentInputCollapsed(true)}
                    >
                      ✕
                    </button>
                  </div>
                </div>
              ) : null}
            </div>
            {controller.agentInputCollapsed ? (
              <button
                type="button"
                className="mcd-agent-toggle"
                title="打开询问框"
                onClick={() => controller.setAgentInputCollapsed(false)}
              >
                ◉
              </button>
            ) : null}
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
