import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import React from "react";
import { Button, Space } from "antd";
import {
  clearAuthToken,
  createShare,
  createDirectory,
  fetchActiveWorkspace,
  fetchCurrentUser,
  deleteDirectory,
  deleteFile,
  downloadFile,
  activateStorageSetting,
  fetchEntriesByParent,
  fetchPlatforms,
  fetchStorageSettings,
  fetchMyShares,
  fetchShareDetail,
  fetchWorkspaces,
  fetchRecycleBin,
  queryAgent,
  fetchAgentHistory,
  streamAgentQuery,
  getAuthToken,
  login,
  logout,
  mergeUpload,
  moveFile,
  precheckUpload,
  rebuildFileIndexes,
  renameDirectory,
  renameFile,
  restoreRecord,
  saveAuthToken,
  updateActiveWorkspace,
  updateActiveStorage,
  updateShare,
  uploadPart,
  setCurrentWorkspaceId,
  setCurrentStorageSettingId
} from "../api/storage.js";
import { DEFAULT_CHUNK_SIZE, ROOT_PARENT_ID } from "../constants/appConfig.js";
import { calculateHash } from "../utils/hash.js";
import { formatBytes } from "../utils/format.js";
import { confirmAction, promptText } from "../utils/dialogs.jsx";

function normalizeWorkspace(item) {
  if (!item || typeof item !== "object") {
    return null;
  }
  return {
    workspaceId: item.workspaceId || item.id || "",
    workspaceName: item.workspaceName || item.name || item.id || "",
    workspaceType: item.workspaceType || "personal",
    role: item.role || "member",
    isDefault: Boolean(item.isDefault)
  };
}

function normalizeStorage(item) {
  if (!item || typeof item !== "object") {
    return null;
  }
  return {
    settingId: item.settingId || item.id || "local-default",
    storageSettingName: item.storageSettingName || item.name || "",
    identifier: item.identifier || item.platformIdentifier || "Local",
    name: item.storageSettingName || item.name || item.identifier || item.platformIdentifier || "Local",
    active: Boolean(item.active),
    basePath: item.basePath || "",
    namespace: item.namespace || "",
    baseUrl: item.baseUrl || "",
    endpoint: item.endpoint || "",
    region: item.region || "",
    bucket: item.bucket || "",
    accessKeyId: item.accessKeyId || "",
    secretAccessKey: item.secretAccessKey || "",
    prefix: item.prefix || "",
    useSSL: Boolean(item.useSSL),
    pathStyle: item.pathStyle !== false
  };
}

function normalizeFileRecord(item) {
  if (!item || typeof item !== "object") {
    return null;
  }
  const directory = Boolean(item.directory ?? item.is_dir ?? item.isDir);
  return {
    fileId: item.fileId || item.id || "",
    fileName: item.fileName || item.name || item.display_name || item.displayName || "",
    fileSize: Number(item.fileSize ?? item.size ?? 0),
    fileHash: item.fileHash || item.contentMd5 || item.content_md5 || "",
    directory,
    raw: item
  };
}

/**
 * 网盘页面控制器。
 *
 * @param {string} normalizedBaseUrl 标准化地址
 * @param {{notifyError: Function, notifySuccess: Function, notifyWarning: Function}} notifier 通知能力
 * @returns {object} 页面状态与动作
 */
export function useCloudDriveController(normalizedBaseUrl, notifier) {
  const { notifyError, notifySuccess, notifyWarning } = notifier;

  const [activeMenu, setActiveMenu] = useState("files");
  const [files, setFiles] = useState([]);
  const [shares, setShares] = useState([]);
  const [agentQuery, setAgentQuery] = useState("");
  const [agentResult, setAgentResult] = useState(null);
  const [agentScope, setAgentScope] = useState("auto");
  const [agentMode, setAgentMode] = useState("search");
  const [agentChatCollapsed, setAgentChatCollapsed] = useState(false);
  const [agentInputCollapsed, setAgentInputCollapsed] = useState(false);
  const [agentRunning, setAgentRunning] = useState(false);
  const [agentHistory, setAgentHistory] = useState([]);
  const [agentHistoryHasMore, setAgentHistoryHasMore] = useState(false);
  const [currentParentId, setCurrentParentId] = useState(ROOT_PARENT_ID);
  const [directoryTrail, setDirectoryTrail] = useState([{ id: ROOT_PARENT_ID, name: "根目录" }]);
  const [platforms, setPlatforms] = useState([]);
  const [workspaces, setWorkspaces] = useState([]);
  const [activeWorkspace, setActiveWorkspace] = useState(null);
  const [activeStorage, setActiveStorage] = useState(null);
  const [storageSettings, setStorageSettings] = useState([]);
  const [enabledStorageIds, setEnabledStorageIds] = useState([]);
  const [currentUser, setCurrentUser] = useState(null);
  const [authenticated, setAuthenticated] = useState(false);
  const [storageForm, setStorageForm] = useState({
    settingId: "local-default",
    storageSettingName: "",
    identifier: "Local",
    basePath: "",
    namespace: "",
    baseUrl: "",
    endpoint: "",
    region: "us-east-1",
    bucket: "",
    accessKeyId: "",
    secretAccessKey: "",
    prefix: "",
    useSSL: false,
    pathStyle: true
  });
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [selectedFile, setSelectedFile] = useState(null);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const previousStorageSettingIdRef = useRef("");
  const enabledStorageKey = useMemo(
    () => `mcd-enabled-storage:${activeWorkspace?.workspaceId || "default"}`,
    [activeWorkspace?.workspaceId]
  );
  const agentScopeOptions = useMemo(() => {
    const options = (storageSettings || []).map((item) => ({
      value: item.settingId,
      label: item.storageSettingName || item.name || item.settingId
    }));
    options.push({ value: "workspace", label: "工作空间" });
    options.push({ value: "auto", label: "自动" });
    return options;
  }, [storageSettings]);

  const loadStorageMeta = useCallback(async () => {
    setError("");
    setLoading(true);
    try {
      const [platformData, settingData] = await Promise.all([
        fetchPlatforms(normalizedBaseUrl),
        fetchStorageSettings(normalizedBaseUrl)
      ]);
      const [workspaceListData, activeWorkspaceData] = await Promise.all([
        fetchWorkspaces(normalizedBaseUrl),
        fetchActiveWorkspace(normalizedBaseUrl)
      ]);
      const platformItems = platformData?.data || platformData || [];
      setPlatforms(Array.isArray(platformItems) ? platformItems : []);

      const workspaceItems = (workspaceListData?.data || workspaceListData || [])
        .map(normalizeWorkspace)
        .filter(Boolean);
      setWorkspaces(workspaceItems);

      const activeWorkspaceRaw = normalizeWorkspace(activeWorkspaceData?.data || activeWorkspaceData);
      const currentWorkspace = activeWorkspaceRaw ?? workspaceItems.find((item) => item.isDefault) ?? workspaceItems[0] ?? null;
      setActiveWorkspace(currentWorkspace);

      const settingItems = (settingData?.data || settingData || [])
        .map(normalizeStorage)
        .filter(Boolean);
      setStorageSettings(settingItems);
      const enabledFromBackend = settingItems.filter((item) => item.active).map((item) => item.settingId);
      const workspaceKey = `mcd-enabled-storage:${currentWorkspace?.workspaceId || "default"}`;
      let enabledFromLocal = [];
      try {
        const raw = window.localStorage.getItem(workspaceKey);
        const parsed = raw ? JSON.parse(raw) : [];
        if (Array.isArray(parsed)) {
          enabledFromLocal = parsed;
        }
      } catch (_) {
        enabledFromLocal = [];
      }
      const validSettingIds = new Set(settingItems.map((item) => item.settingId));
      const mergedEnabled = [...new Set([...enabledFromBackend, ...enabledFromLocal])].filter((id) => validSettingIds.has(id));
      setEnabledStorageIds(mergedEnabled);
      const active = settingItems.find((item) => item.active) || settingItems[0] || null;
      setActiveStorage(active);
      setStorageForm({
        settingId: active?.settingId || "local-default",
        storageSettingName: active?.storageSettingName || active?.name || "",
        identifier: active?.identifier || "Local",
        basePath: active?.basePath || "",
        namespace: active?.namespace || "",
        baseUrl: active?.baseUrl || "",
        endpoint: active?.endpoint || "",
        region: active?.region || "us-east-1",
        bucket: active?.bucket || "",
        accessKeyId: active?.accessKeyId || "",
        secretAccessKey: active?.secretAccessKey || "",
        prefix: active?.prefix || "",
        useSSL: Boolean(active?.useSSL),
        pathStyle: active?.pathStyle !== false
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl]);

  useEffect(() => {
    try {
      window.localStorage.setItem(enabledStorageKey, JSON.stringify(enabledStorageIds));
    } catch (_) {
      // 忽略本地缓存异常。
    }
  }, [enabledStorageIds, enabledStorageKey]);

  const checkAuth = useCallback(async () => {
    const tokenSnapshot = getAuthToken();
    try {
      const response = await fetchCurrentUser(normalizedBaseUrl);
      const user = response?.data || response;
      setCurrentUser({
        ...user,
        displayName: user?.nickname || user?.displayName || user?.username
      });
      setAuthenticated(true);
      return true;
    } catch (_) {
      if (tokenSnapshot && tokenSnapshot === getAuthToken()) {
        clearAuthToken();
      }
      setCurrentUser(null);
      setAuthenticated(false);
      return false;
    }
  }, [normalizedBaseUrl]);

  const handleLogin = useCallback(async (username, password) => {
    setError("");
    setLoading(true);
    try {
      const response = await login(normalizedBaseUrl, { username, password });
      const payload = response.data || response;
      saveAuthToken(payload.token);
      const isAuthed = await checkAuth();
      if (!isAuthed) {
        throw new Error("登录校验失败，请重试");
      }
      setCurrentParentId(ROOT_PARENT_ID);
      setDirectoryTrail([{ id: ROOT_PARENT_ID, name: "根目录" }]);
      await loadStorageMeta();
      notifySuccess("登录成功");
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : "登录失败");
      return false;
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl, checkAuth, loadStorageMeta, notifySuccess]);

  const handleLogout = useCallback(async () => {
    try {
      await logout(normalizedBaseUrl);
    } catch (_) {
      // 忽略服务端登出异常，前端仍执行本地清理。
    }
    clearAuthToken();
    setCurrentUser(null);
    setAuthenticated(false);
    setFiles([]);
    setActiveStorage(null);
    setActiveWorkspace(null);
    notifySuccess("已退出登录");
  }, [normalizedBaseUrl, notifySuccess]);

  const loadFilesByParent = useCallback(async (parentId) => {
    setError("");
    setLoading(true);
    try {
      const result = await fetchEntriesByParent(normalizedBaseUrl, parentId);
      const payload = result?.data || result;
      const items = payload?.items || payload || [];
      setFiles((Array.isArray(items) ? items : []).map(normalizeFileRecord).filter(Boolean));
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl]);

  const loadFiles = useCallback(async () => {
    await loadFilesByParent(currentParentId);
  }, [loadFilesByParent, currentParentId]);

  const loadRecycleBin = useCallback(async () => {
    setError("");
    setLoading(true);
    try {
      const result = await fetchRecycleBin(normalizedBaseUrl);
      const payload = result?.data || result;
      const items = payload?.items || payload || [];
      setFiles((Array.isArray(items) ? items : []).map(normalizeFileRecord).filter(Boolean));
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载回收站失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl]);

  const loadShares = useCallback(async () => {
    setError("");
    setLoading(true);
    try {
      const result = await fetchMyShares(normalizedBaseUrl);
      const items = result?.data || result || [];
      setShares(Array.isArray(items) ? items : []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载分享列表失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl]);

  const openDirectory = useCallback((record) => {
    if (!record?.directory) {
      return;
    }
    setCurrentParentId(record.fileId);
    setDirectoryTrail((previous) => [...previous, { id: record.fileId, name: record.fileName }]);
  }, []);

  const jumpToDirectory = useCallback((index) => {
    setDirectoryTrail((previous) => {
      const nextTrail = previous.slice(0, index + 1);
      const last = nextTrail[nextTrail.length - 1];
      setCurrentParentId(last?.id || ROOT_PARENT_ID);
      return nextTrail;
    });
  }, []);

  const updateStorageFormField = useCallback((key, value) => {
    setStorageForm((previous) => ({
      ...previous,
      [key]: value
    }));
  }, []);

  const handleCreateFolder = useCallback(async () => {
    const folderName = await promptText({
      title: "新建目录",
      label: "请输入目录名称",
      placeholder: "例如：项目资料",
      required: true
    });
    if (!folderName) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      await createDirectory(normalizedBaseUrl, {
        name: folderName,
        parentId: currentParentId
      });
      await loadFiles();
      notifySuccess("目录创建成功");
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建目录失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl, currentParentId, loadFiles, notifySuccess]);

  const handleRenameFolder = useCallback(async (record) => {
    if (!record?.directory) {
      return;
    }
    const nextName = await promptText({
      title: "重命名目录",
      label: "目录新名称",
      initialValue: record.fileName,
      required: true
    });
    if (!nextName) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      await renameDirectory(normalizedBaseUrl, record.fileId, nextName);
      await loadFiles();
      notifySuccess("目录重命名成功");
    } catch (err) {
      setError(err instanceof Error ? err.message : "重命名目录失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl, loadFiles, notifySuccess]);

  const handleRenameFile = useCallback(async (record) => {
    if (record?.directory) {
      return;
    }
    const nextName = await promptText({
      title: "重命名文件",
      label: "文件新名称",
      initialValue: record.fileName,
      required: true
    });
    if (!nextName) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      await renameFile(normalizedBaseUrl, record.fileId, nextName);
      await loadFiles();
      notifySuccess("文件重命名成功");
    } catch (err) {
      setError(err instanceof Error ? err.message : "重命名文件失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl, loadFiles, notifySuccess]);

  const handleMoveFile = useCallback(async (record) => {
    if (record?.directory) {
      return;
    }
    const targetParentId = await promptText({
      title: "移动文件",
      label: "目标目录ID",
      initialValue: ROOT_PARENT_ID,
      required: true
    });
    if (!targetParentId) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      await moveFile(normalizedBaseUrl, record.fileId, targetParentId);
      await loadFiles();
      notifySuccess("文件移动成功");
    } catch (err) {
      setError(err instanceof Error ? err.message : "移动文件失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl, loadFiles, notifySuccess]);

  const handleDeleteFolder = useCallback(async (record) => {
    if (!record?.directory) {
      return;
    }
    const confirmed = await confirmAction({
      title: "删除目录",
      content: `将目录「${record.fileName}」移动到回收站，是否继续？`,
      okText: "移动到回收站",
      cancelText: "取消",
      okType: "danger"
    });
    if (!confirmed) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      await deleteDirectory(normalizedBaseUrl, record.fileId, true, true);
      await loadFiles();
      notifySuccess("目录删除成功");
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除目录失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl, loadFiles, notifySuccess]);

  const handleDeleteFile = useCallback(async (record) => {
    if (record?.directory) {
      return;
    }
    const softDelete = await confirmAction({
      title: "删除文件",
      content: `将文件「${record.fileName}」移动到回收站，是否继续？`,
      okText: "移动到回收站",
      cancelText: "取消",
      okType: "danger"
    });
    if (!softDelete) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      await deleteFile(normalizedBaseUrl, record.fileId, softDelete);
      await loadFiles();
      notifySuccess("文件删除成功");
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除文件失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl, loadFiles, notifySuccess]);

  const handleDownloadFile = useCallback(async (record) => {
    if (!record || record.directory) {
      return;
    }
    setError("");
    try {
      await downloadFile(normalizedBaseUrl, record.fileId);
      notifySuccess("开始下载");
    } catch (err) {
      setError(err instanceof Error ? err.message : "下载失败");
    }
  }, [normalizedBaseUrl, notifySuccess]);

  const handleRestore = useCallback(async (record) => {
    setError("");
    setLoading(true);
    try {
      await restoreRecord(normalizedBaseUrl, record.fileId);
      await loadRecycleBin();
      notifySuccess("恢复成功");
    } catch (err) {
      setError(err instanceof Error ? err.message : "恢复失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl, loadRecycleBin, notifySuccess]);

  const handleCreateShare = useCallback(async (record) => {
    if (!record || record.directory) {
      return;
    }
    const shareName = await promptText({
      title: "创建分享",
      label: "分享名称",
      initialValue: `${record.fileName} 的分享`,
      required: true
    });
    if (shareName == null) {
      return;
    }
    const shareCodeInput = await promptText({
      title: "创建分享",
      label: "提取码（留空自动生成）",
      initialValue: "",
      placeholder: "例如：AB12CD",
      required: false
    });
    if (shareCodeInput == null) {
      return;
    }
    const expireHoursInput = await promptText({
      title: "创建分享",
      label: "有效期小时（留空表示不过期）",
      initialValue: "24",
      required: false
    });
    if (expireHoursInput == null) {
      return;
    }

    let expireSeconds;
    const trimmedExpire = expireHoursInput.trim();
    if (trimmedExpire) {
      const parsedHours = Number(trimmedExpire);
      if (!Number.isFinite(parsedHours) || parsedHours <= 0) {
        notifyWarning("有效期小时数必须是正数");
        return;
      }
      expireSeconds = Math.floor(parsedHours * 3600);
    }

    setError("");
    setLoading(true);
    try {
      const result = await createShare(normalizedBaseUrl, {
        shareName: shareName || `${record.fileName} 的分享`,
        fileIds: [record.fileId],
        shareCode: shareCodeInput || undefined,
        expireSeconds
      });
      const payload = result.data || result;
      const shareCode = payload?.shareCode || "(自动生成失败)";
      const shareLink = `${window.location.origin}${payload?.accessPath || ""}`;
      if (navigator?.clipboard?.writeText) {
        try {
          await navigator.clipboard.writeText(`分享ID: ${payload.shareId}\n提取码: ${shareCode}\n访问接口: ${shareLink}`);
        } catch (_) {
          // 剪贴板失败不影响主流程。
        }
      }
      notifySuccess(`分享已创建：${payload.shareId}（提取码：${shareCode}）`);
      await loadShares();
      setActiveMenu("shares");
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建分享失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl, loadShares, notifySuccess, notifyWarning]);

  const handleAccessShare = useCallback(async (record) => {
    if (!record?.shareId) {
      return;
    }
    const inputCode = await promptText({
      title: "打开分享页面",
      label: "提取码（可选，留空直接打开）",
      initialValue: record.shareCode || "",
      required: false
    });
    if (inputCode == null) {
      return;
    }
    const sharePath = `/share/${record.shareId}`;
    const code = inputCode.trim();
    const finalURL = code
      ? `${window.location.origin}${sharePath}?code=${encodeURIComponent(code)}`
      : `${window.location.origin}${sharePath}`;
    window.open(finalURL, "_blank", "noopener,noreferrer");
  }, []);

  const handleEditShare = useCallback(async (record) => {
    if (!record?.shareId) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      const detailResp = await fetchShareDetail(normalizedBaseUrl, record.shareId);
      const detail = detailResp.data || detailResp;

      const nextName = await promptText({
        title: "编辑分享",
        label: "分享名称",
        initialValue: detail.shareName || "",
        required: true
      });
      if (nextName == null) {
        return;
      }
      const nextCode = await promptText({
        title: "编辑分享",
        label: "提取码（留空自动生成）",
        initialValue: detail.shareCode || "",
        required: false
      });
      if (nextCode == null) {
        return;
      }
      const now = Date.now();
      const currentExpireHours = detail.expireTime
        ? Math.max(1, Math.round((new Date(detail.expireTime).getTime() - now) / 3600000))
        : "";
      const nextExpireHours = await promptText({
        title: "编辑分享",
        label: "有效期小时（留空=不过期）",
        initialValue: String(currentExpireHours),
        required: false
      });
      if (nextExpireHours == null) {
        return;
      }
      const fileIdsText = await promptText({
        title: "编辑分享",
        label: "文件ID列表（英文逗号分隔）",
        initialValue: (detail.fileIds || []).join(","),
        required: true
      });
      if (fileIdsText == null) {
        return;
      }
      const allowDownload = await confirmAction({
        title: "编辑分享",
        content: "是否允许下载？选择“允许”后公开页可直接下载。",
        okText: "允许下载",
        cancelText: "仅预览"
      });

      const fileIds = fileIdsText.split(",").map((item) => item.trim()).filter(Boolean);
      if (fileIds.length === 0) {
        notifyWarning("至少保留一个文件");
        return;
      }
      const trimmedExpire = nextExpireHours.trim();
      let expireSeconds;
      if (trimmedExpire) {
        const parsed = Number(trimmedExpire);
        if (!Number.isFinite(parsed) || parsed <= 0) {
          notifyWarning("有效期小时必须是正数");
          return;
        }
        expireSeconds = Math.floor(parsed * 3600);
      }

      await updateShare(normalizedBaseUrl, record.shareId, {
        shareName: nextName || detail.shareName,
        shareCode: nextCode,
        expireSeconds,
        allowDownload,
        fileIds
      });
      notifySuccess("分享已更新");
      await loadShares();
    } catch (err) {
      setError(err instanceof Error ? err.message : "编辑分享失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl, loadShares, notifySuccess, notifyWarning]);

  const handleApplyStorageConfig = useCallback(async () => {
    setError("");
    setLoading(true);
    try {
      await updateActiveStorage(normalizedBaseUrl, {
        settingId: storageForm.settingId,
        storageSettingName: storageForm.storageSettingName,
        identifier: storageForm.identifier,
        basePath: storageForm.basePath,
        namespace: storageForm.namespace,
        baseUrl: storageForm.baseUrl,
        endpoint: storageForm.endpoint,
        region: storageForm.region,
        bucket: storageForm.bucket,
        accessKeyId: storageForm.accessKeyId,
        secretAccessKey: storageForm.secretAccessKey,
        prefix: storageForm.prefix,
        useSSL: storageForm.useSSL,
        pathStyle: storageForm.pathStyle
      });
      setCurrentParentId(ROOT_PARENT_ID);
      setDirectoryTrail([{ id: ROOT_PARENT_ID, name: "根目录" }]);
      await loadStorageMeta();
      if (activeMenu === "files") {
        await loadFilesByParent(ROOT_PARENT_ID);
      }
      if (activeMenu === "trash") {
        await loadRecycleBin();
      }
      notifySuccess("存储配置已应用");
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存存储配置失败");
    } finally {
      setLoading(false);
    }
  }, [storageForm, normalizedBaseUrl, loadStorageMeta, activeMenu, loadFilesByParent, loadRecycleBin, notifySuccess]);

  const handleEditStorageSetting = useCallback((settingId) => {
    if (!settingId) {
      return;
    }
    const target = storageSettings.find((item) => item.settingId === settingId);
    if (!target) {
      return;
    }
    setStorageForm({
      settingId: target.settingId || "",
      storageSettingName: target.storageSettingName || target.name || "",
      identifier: target.identifier || "Local",
      basePath: target.basePath || "",
      namespace: target.namespace || "",
      baseUrl: target.baseUrl || "",
      endpoint: target.endpoint || "",
      region: target.region || "us-east-1",
      bucket: target.bucket || "",
      accessKeyId: target.accessKeyId || "",
      secretAccessKey: target.secretAccessKey || "",
      prefix: target.prefix || "",
      useSSL: Boolean(target.useSSL),
      pathStyle: target.pathStyle !== false
    });
    notifySuccess(`已载入配置：${target.settingId}`);
  }, [storageSettings, notifySuccess]);

  const handleCreateStorageDraft = useCallback((draft = null) => {
    const normalizedIdentifier = draft?.identifier || "Local";
    setStorageForm((previous) => ({
      ...previous,
      settingId: "",
      storageSettingName: draft?.storageSettingName || "",
      identifier: normalizedIdentifier,
      basePath: "",
      namespace: draft?.namespace || "",
      baseUrl: draft?.baseUrl || "",
      endpoint: draft?.endpoint || "",
      region: draft?.region || "us-east-1",
      bucket: draft?.bucket || "",
      accessKeyId: "",
      secretAccessKey: "",
      prefix: "",
      useSSL: false,
      pathStyle: true
    }));
    notifySuccess("已进入新建配置编辑页，请补全参数后保存");
  }, [notifySuccess]);

  const handleActivateStorageSetting = useCallback(async (settingId) => {
    if (!settingId) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      await activateStorageSetting(normalizedBaseUrl, settingId);
      await loadStorageMeta();
      if (activeMenu === "files") {
        await loadFilesByParent(ROOT_PARENT_ID);
      }
      if (activeMenu === "trash") {
        await loadRecycleBin();
      }
      notifySuccess(`已启用配置：${settingId}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "启用配置失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl, loadStorageMeta, activeMenu, loadFilesByParent, loadRecycleBin, notifySuccess]);

  const handleEnableStorageSetting = useCallback((settingId) => {
    if (!settingId) {
      return;
    }
    setEnabledStorageIds((previous) => {
      if (previous.includes(settingId)) {
        return previous;
      }
      return [...previous, settingId];
    });
    notifySuccess(`已加入可访问配置：${settingId}`);
  }, [notifySuccess]);

  const handleDisableStorageSetting = useCallback((settingId) => {
    if (!settingId) {
      return;
    }
    if ((enabledStorageIds || []).length <= 1 && (enabledStorageIds || []).includes(settingId)) {
      notifyWarning("不能全部关闭，至少保留一个已启用配置");
      return;
    }
    setEnabledStorageIds((previous) => previous.filter((id) => id !== settingId));
    notifySuccess(`已从可访问配置移除：${settingId}`);
  }, [enabledStorageIds, notifySuccess, notifyWarning]);

  const handleSwitchWorkspace = useCallback(async (workspaceId) => {
    if (!workspaceId || workspaceId === activeWorkspace?.workspaceId) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      await updateActiveWorkspace(normalizedBaseUrl, { workspaceId });
      const switched = workspaces.find((item) => item.workspaceId === workspaceId) || { workspaceId };
      setActiveWorkspace(switched);
      setCurrentParentId(ROOT_PARENT_ID);
      setDirectoryTrail([{ id: ROOT_PARENT_ID, name: "根目录" }]);
      await loadStorageMeta();
      if (activeMenu === "files") {
        await loadFilesByParent(ROOT_PARENT_ID);
      }
      if (activeMenu === "trash") {
        await loadRecycleBin();
      }
      notifySuccess(`已切换到空间：${switched.workspaceName || switched.workspaceId}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "切换工作空间失败");
    } finally {
      setLoading(false);
    }
  }, [activeWorkspace, workspaces, normalizedBaseUrl, loadStorageMeta, activeMenu, loadFilesByParent, loadRecycleBin, notifySuccess]);

  const handleRebuildIndexes = useCallback(async () => {
    const confirmed = window.confirm("确认重建 mcd_file_record 索引？\n该操作会短暂占用数据库资源。");
    if (!confirmed) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      const response = await rebuildFileIndexes(normalizedBaseUrl);
      const payload = response?.data || response;
      const rebuild = payload?.rebuild || payload;
      const reconcile = payload?.reconcile || null;
      if (activeMenu === "files") {
        await loadFiles();
      }
      if (activeMenu === "trash") {
        await loadRecycleBin();
      }
      notifySuccess(
        `索引重建完成（表: ${rebuild.tableName}，删:${(rebuild.droppedIndexes || []).length}，建:${(rebuild.createdIndexes || []).length}，修复:${rebuild.repairedRows || 0}`
        + `${reconcile ? `，校准: 删${reconcile.markedDeletedRows} / 增${reconcile.insertedRows} / 改${reconcile.updatedRows}` : ""}）`
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "重建索引失败");
    } finally {
      setLoading(false);
    }
  }, [normalizedBaseUrl, activeMenu, loadFiles, loadRecycleBin, notifySuccess]);

  const handleCreateWorkspace = useCallback((draft = null) => {
    const name = draft?.workspaceName?.trim();
    if (!name) {
      notifyWarning("请先填写空间名称");
      return;
    }
    notifyWarning(`新建空间接口待后端提供（已提交草稿：${name}）`);
  }, [notifyWarning]);

  const handleRenameWorkspace = useCallback(() => {
    notifyWarning("重命名空间接口待后端提供");
  }, [notifyWarning]);

  const handleDeleteWorkspace = useCallback(() => {
    notifyWarning("删除空间接口待后端提供");
  }, [notifyWarning]);

  const handleAddWorkspaceUser = useCallback(() => {
    notifyWarning("空间成员添加接口待后端提供");
  }, [notifyWarning]);

  const handleRemoveWorkspaceUser = useCallback(() => {
    notifyWarning("空间成员移除接口待后端提供");
  }, [notifyWarning]);

  const handleDeleteStorageSetting = useCallback(() => {
    notifyWarning("删除存储配置接口待后端提供");
  }, [notifyWarning]);

  const loadAgentHistory = useCallback(async (before = "") => {
    try {
      const result = await fetchAgentHistory(normalizedBaseUrl, { before, size: 10 });
      if (!before) {
        setAgentHistory(result.items || []);
      } else {
        setAgentHistory((prev) => [...prev, ...(result.items || [])]);
      }
      setAgentHistoryHasMore(result.hasMore !== false);
    } catch (_) {
      // 静默失败，历史加载不影响主功能
    }
  }, [normalizedBaseUrl]);

  const handleAgentQuery = useCallback(async () => {
    if (agentRunning) {
      return;
    }
    const q = String(agentQuery || "").trim();
    if (!q) {
      notifyWarning("请输入检索问题");
      return;
    }
    setError("");
    setLoading(true);
    setAgentRunning(true);

    const selectedScope = String(agentScope || "").trim();
    let scope = "auto";
    let storageSettingId = activeStorage?.settingId || "";
    if (selectedScope === "workspace") {
      scope = "workspace";
      storageSettingId = "";
    } else if (selectedScope === "auto") {
      scope = "auto";
    } else if (selectedScope) {
      scope = "storage_setting";
      storageSettingId = selectedScope;
    }
    const payload = {
      query: q,
      scope,
      mode: agentMode,
      workspaceId: activeWorkspace?.workspaceId || "",
      storageSettingId
    };

    // 所有模式都用 SSE 流式输出，search 模式也一样
    setAgentResult(null);
    streamAgentQuery(normalizedBaseUrl, payload, (event, data) => {
      if (event === "error") {
        setError(data.message || "Agent 检索失败");
        setAgentRunning(false);
        setLoading(false);
        return;
      }
      if (event === "start") {
        setAgentResult((prev) => ({ ...prev, traceId: data.traceId || "streaming...", provider: data.provider, model: data.model, summary: "正在处理..." }));
      }
      if (event === "llm.decide.done") {
        setAgentResult((prev) => ({ ...prev, intent: data.intent, routeMode: "llm" }));
      }
      if (event === "tool.start") {
        setAgentResult((prev) => ({ ...prev, summary: "正在调用工具: " + (data.tool || "") }));
      }
      if (event === "tool.done") {
        setAgentResult((prev) => {
          const items = [...(prev?.items || []), ...(data.items || [])];
          const sources = [...(prev?.sources || []), data.source].filter(Boolean);
          return { ...prev, items, sources };
        });
      }
      if (event === "summary.token") {
        setAgentResult((prev) => ({ ...prev, summary: data.summary }));
      }
      if (event === "summarize.done") {
        setAgentResult((prev) => ({ ...prev, summary: data.summary }));
      }
      if (event === "plan") {
        setAgentResult((prev) => ({ ...prev, summary: data.summary, executionPlan: data }));
      }
      if (event === "confirm.required") {
        setAgentResult((prev) => ({ ...prev, waitingConfirm: true }));
        setAgentRunning(false);
        setLoading(false);
      }
      if (event === "done") {
        setAgentRunning(false);
        setLoading(false);
        notifySuccess("Agent 检索完成");
        loadAgentHistory();
      }
    });
  }, [agentRunning, agentQuery, agentScope, agentMode, normalizedBaseUrl, activeWorkspace, activeStorage, notifyWarning, notifySuccess, loadAgentHistory]);

  useEffect(() => {
    if (activeMenu === "knowledge") {
      setAgentScope("workspace");
      return;
    }
    if (activeMenu === "files" || activeMenu === "trash") {
      setAgentScope(activeStorage?.settingId || "auto");
      return;
    }
    setAgentScope("auto");
  }, [activeMenu, activeStorage]);

  const handleUpload = useCallback(async () => {
    if (!selectedFile) {
      notifyWarning("请先选择文件");
      return;
    }
    setError("");
    setUploadProgress(0);
    setLoading(true);
    try {
      const fileHash = await calculateHash(selectedFile);
      const totalParts = Math.ceil(selectedFile.size / DEFAULT_CHUNK_SIZE);
      const precheckResult = await precheckUpload(normalizedBaseUrl, {
        fileName: selectedFile.name,
        fileSize: selectedFile.size,
        fileHash,
        totalParts,
        contentType: selectedFile.type,
        parentId: currentParentId
      });

      const precheckData = precheckResult.data || precheckResult;
      if (precheckData.skipUpload) {
        setUploadProgress(100);
        await loadFiles();
        notifySuccess("秒传成功");
        return;
      }

      const taskId = precheckData.taskId || precheckData.uploadId;
      if (!taskId) {
        throw new Error("后端未返回 taskId/uploadId");
      }
      for (let partNumber = 1; partNumber <= totalParts; partNumber += 1) {
        const start = (partNumber - 1) * DEFAULT_CHUNK_SIZE;
        const end = Math.min(selectedFile.size, partNumber * DEFAULT_CHUNK_SIZE);
        const chunk = selectedFile.slice(start, end);
        const chunkMd5 = await calculateHash(chunk);
        await uploadPart(normalizedBaseUrl, {
          taskId,
          uploadId: taskId,
          chunkIndex: partNumber,
          partNumber,
          totalParts,
          fileHash,
          chunkMd5,
          file: chunk
        });
        setUploadProgress(Math.round((partNumber / totalParts) * 100));
      }

      await mergeUpload(normalizedBaseUrl, { taskId, uploadId: taskId });
      setUploadProgress(100);
      await loadFiles();
      notifySuccess("文件上传完成");
    } catch (err) {
      setError(err instanceof Error ? err.message : "上传失败");
    } finally {
      setLoading(false);
    }
  }, [selectedFile, normalizedBaseUrl, currentParentId, loadFiles, notifyWarning, notifySuccess]);

  const columns = useMemo(() => {
    const renderFileName = (value, record) => {
      if (record.directory) {
        return (
          <button type="button" className="text-blue-600 hover:underline" onClick={() => openDirectory(record)}>
            📁 {value}
          </button>
        );
      }
      return value;
    };

    const renderFileSize = (value, record) => (record?.directory ? "-" : formatBytes(value));

    const renderFileHash = (value, record) => (record?.directory ? <span className="mcd-muted">目录</span> : <span className="mcd-muted">{value}</span>);

    return [
      { title: "文件名", dataIndex: "fileName", key: "fileName", render: renderFileName },
      { title: "大小", dataIndex: "fileSize", key: "fileSize", render: renderFileSize },
      { title: "哈希", dataIndex: "fileHash", key: "fileHash", render: renderFileHash },
      {
        title: "操作",
        key: "actions",
        render: (_, record) => (
          activeMenu === "trash" ? (
            <Button size="small" onClick={() => handleRestore(record)}>恢复</Button>
          ) : record.directory ? (
            <Space>
              <Button size="small" onClick={() => handleRenameFolder(record)}>重命名</Button>
              <Button size="small" danger onClick={() => handleDeleteFolder(record)}>删除</Button>
            </Space>
          ) : (
            <Space>
              <Button size="small" onClick={() => handleRenameFile(record)}>重命名</Button>
              <Button size="small" onClick={() => handleMoveFile(record)}>移动</Button>
              <Button size="small" onClick={() => handleDownloadFile(record)}>下载</Button>
              <Button size="small" type="primary" ghost onClick={() => handleCreateShare(record)}>分享</Button>
              <Button size="small" danger onClick={() => handleDeleteFile(record)}>删除</Button>
            </Space>
          )
        )
      }
    ];
  }, [activeMenu, openDirectory, handleRestore, handleRenameFolder, handleDeleteFolder, handleRenameFile, handleMoveFile, handleDownloadFile, handleCreateShare, handleDeleteFile]);

  const shareColumns = useMemo(() => ([
    { title: "分享ID", dataIndex: "shareId", key: "shareId" },
    { title: "名称", dataIndex: "shareName", key: "shareName" },
    {
      title: "归属空间",
      key: "workspace",
      render: (_, record) => record.workspaceName || record.workspaceId || "-"
    },
    {
      title: "归属配置",
      key: "setting",
      render: (_, record) => record.storageSettingName || record.storageSettingId || "-"
    },
    { title: "提取码", dataIndex: "shareCode", key: "shareCode", render: (value) => value || "无" },
    { title: "下载", dataIndex: "allowDownload", key: "allowDownload", render: (value) => (value ? "允许" : "禁止") },
    {
      title: "访问次数",
      key: "views",
      render: (_, record) => `${record.viewCount || 0} / 下载 ${record.downloadCount || 0}`
    },
    {
      title: "状态",
      key: "status",
      render: (_, record) => (record.status === 0 ? "生效中" : "已失效")
    },
    {
      title: "操作",
      key: "actions",
      render: (_, record) => (
        <Space>
          <Button
            size="small"
            onClick={async () => {
              const link = `${window.location.origin}${record.accessPath}`;
              if (navigator?.clipboard?.writeText) {
                try {
                  await navigator.clipboard.writeText(link);
                  notifySuccess("分享链接已复制");
                  return;
                } catch (_) {
                  // fallback
                }
              }
              window.prompt("复制分享链接", link);
            }}
          >
            查看链接
          </Button>
          <Button size="small" onClick={() => handleEditShare(record)}>
            编辑
          </Button>
          <Button size="small" type="primary" onClick={() => handleAccessShare(record)}>
            打开页面
          </Button>
        </Space>
      )
    }
  ]), [handleEditShare, handleAccessShare, notifySuccess]);

  useEffect(() => {
    setCurrentWorkspaceId(activeWorkspace?.workspaceId || "");
  }, [activeWorkspace]);

  useEffect(() => {
    setCurrentStorageSettingId(activeStorage?.settingId || "");
  }, [activeStorage]);

  useEffect(() => {
    if (!authenticated) {
      return;
    }
    if (activeMenu === "files") {
      loadFiles();
      return;
    }
    if (activeMenu === "trash") {
      loadRecycleBin();
      return;
    }
    if (activeMenu === "shares") {
      loadShares();
      return;
    }
    if (activeMenu === "agent" && agentResult == null) {
      setAgentResult(null);
    }
  }, [authenticated, activeMenu, loadFiles, loadRecycleBin, loadShares, agentResult]);

  useEffect(() => {
    const currentSettingId = activeStorage?.settingId || "";
    if (!currentSettingId) {
      return;
    }
    if (previousStorageSettingIdRef.current === currentSettingId) {
      return;
    }
    previousStorageSettingIdRef.current = currentSettingId;
    setCurrentParentId(ROOT_PARENT_ID);
    setDirectoryTrail([{ id: ROOT_PARENT_ID, name: "根目录" }]);
    if (activeMenu === "files") {
      loadFilesByParent(ROOT_PARENT_ID);
      return;
    }
    if (activeMenu === "trash") {
      loadRecycleBin();
    }
  }, [activeStorage, activeMenu, loadFilesByParent, loadRecycleBin]);

  useEffect(() => {
    (async () => {
      const isAuthed = await checkAuth();
      if (isAuthed) {
        await loadStorageMeta();
        loadAgentHistory();
      }
    })();
  }, [checkAuth, loadStorageMeta, loadAgentHistory]);

  useEffect(() => {
    if (!error) {
      return;
    }
    notifyError(error);
    setError("");
  }, [error, notifyError]);

  return {
    activeMenu,
    files,
    shares,
    agentQuery,
    agentResult,
    agentScope,
    agentScopeOptions,
    agentMode,
    agentChatCollapsed,
    agentInputCollapsed,
    agentRunning,
    currentParentId,
    directoryTrail,
    platforms,
    currentUser,
    authenticated,
    activeStorage,
    workspaces,
    activeWorkspace,
    storageForm,
    storageSettings,
    enabledStorageIds,
    drawerOpen,
    selectedFile,
    uploadProgress,
    loading,
    columns,
    shareColumns,
    setActiveMenu,
    setAgentQuery,
    setAgentScope,
    setAgentMode,
    setAgentChatCollapsed,
    setAgentInputCollapsed,
    setDrawerOpen,
    setSelectedFile,
    loadStorageMeta,
    handleLogin,
    handleLogout,
    loadFiles,
    loadRecycleBin,
    loadShares,
    jumpToDirectory,
    handleCreateFolder,
    handleUpload,
    updateStorageFormField,
    handleApplyStorageConfig,
    handleEditStorageSetting,
    handleCreateStorageDraft,
    handleActivateStorageSetting,
    handleEnableStorageSetting,
    handleDisableStorageSetting,
    handleSwitchWorkspace,
    handleRebuildIndexes,
    handleCreateWorkspace,
    handleRenameWorkspace,
    handleDeleteWorkspace,
    handleAddWorkspaceUser,
    handleRemoveWorkspaceUser,
    handleDeleteStorageSetting,
    handleAgentQuery,
    handleBaseFileActions: {
      renameFolder: handleRenameFolder,
      deleteFolder: handleDeleteFolder,
      renameFile: handleRenameFile,
      moveFile: handleMoveFile,
      deleteFile: handleDeleteFile,
      restore: handleRestore
    },
    agentHistory,
    agentHistoryHasMore,
    loadAgentHistory,
    handleCreateShare,
    handleAccessShare,
    handleEditShare
  };
}
