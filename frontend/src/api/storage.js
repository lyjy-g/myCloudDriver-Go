/**
 * 前端 API 适配层。
 * 目标：无后端改动前提下，适配当前 Go 路由与响应结构。
 */

export const AUTH_TOKEN_KEY = "mcd-console-auth-token";

export function saveAuthToken(token) {
  if (!token) {
    return;
  }
  window.localStorage.setItem(AUTH_TOKEN_KEY, token);
}

export function getAuthToken() {
  return window.localStorage.getItem(AUTH_TOKEN_KEY) || "";
}

export function clearAuthToken() {
  window.localStorage.removeItem(AUTH_TOKEN_KEY);
}

function createIdempotencyKey() {
  if (typeof crypto !== "undefined" && crypto?.randomUUID) {
    return crypto.randomUUID();
  }
  return `idem_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

function buildHeaders(headers = {}, withAuth = true, withIdempotency = false) {
  const finalHeaders = { ...headers };
  if (withAuth) {
    const token = getAuthToken();
    if (token) {
      finalHeaders.Authorization = `Bearer ${token}`;
    }
  }
  if (withIdempotency) {
    finalHeaders["Idempotency-Key"] = createIdempotencyKey();
  }
  return finalHeaders;
}

function unwrapPayload(payload) {
  if (payload == null || typeof payload !== "object") {
    return payload;
  }
  if (Object.prototype.hasOwnProperty.call(payload, "data")) {
    return payload.data;
  }
  return payload;
}

function normalizeIdentifier(identifier = "") {
  return String(identifier || "").trim().toLowerCase();
}

function parseSettingConfig(item) {
  const rawConfig = item?.configJson || item?.configData || "{}";
  if (typeof rawConfig === "string") {
    try {
      return JSON.parse(rawConfig || "{}");
    } catch {
      return {};
    }
  }
  if (rawConfig && typeof rawConfig === "object") {
    return rawConfig;
  }
  return {};
}

function readErrorMessage(payload, fallback = "请求失败") {
  if (!payload || typeof payload !== "object") {
    return fallback;
  }
  return payload.msg || payload.message || payload.error || fallback;
}

function isSuccessCode(code) {
  return code === undefined || code === 200 || code === "200" || code === "OK";
}

async function requestJson(method, urls, {
  body,
  withAuth = true,
  withIdempotency = false
} = {}) {
  const candidates = Array.isArray(urls) ? urls : [urls];
  let lastError = null;

  for (const url of candidates) {
    const response = await fetch(url, {
      method,
      headers: buildHeaders(
        body === undefined
          ? { Accept: "application/json" }
          : { Accept: "application/json", "Content-Type": "application/json" },
        withAuth,
        withIdempotency
      ),
      body: body === undefined ? undefined : JSON.stringify(body)
    });

    // 404/405 尝试下一个兼容路径。
    if (response.status === 404 || response.status === 405) {
      lastError = new Error(`endpoint not found: ${url}`);
      continue;
    }

    const text = await response.text();
    let payload = null;
    if (text) {
      try {
        payload = JSON.parse(text);
      } catch {
        payload = null;
      }
    }

    if (!response.ok) {
      throw new Error(readErrorMessage(payload, `请求失败(${response.status})`));
    }

    if (payload && Object.prototype.hasOwnProperty.call(payload, "code") && !isSuccessCode(payload.code)) {
      throw new Error(readErrorMessage(payload));
    }

    if (payload && payload.success === false) {
      throw new Error(readErrorMessage(payload));
    }

    return payload;
  }

  throw lastError || new Error("请求失败");
}

export function getJson(url, withAuth = true) {
  return requestJson("GET", url, { withAuth });
}

export function postJson(url, payload, withAuth = true) {
  return requestJson("POST", url, { body: payload, withAuth });
}

export function putJson(url, payload, withAuth = true) {
  return requestJson("PUT", url, { body: payload, withAuth });
}

export function fetchPlatforms(baseUrl) {
  return requestJson("GET", [
    `${baseUrl}/api/v1/storage/platforms`,
    `${baseUrl}/apis/storage/platforms`
  ]);
}

export async function fetchStorageSettings(baseUrl) {
  const listResp = await requestJson("GET", [
    `${baseUrl}/api/v1/storage/settings`,
    `${baseUrl}/apis/storage/platform/settings`,
    `${baseUrl}/apis/storage/settings`
  ]);
  const settings = unwrapPayload(listResp) || [];
  if (!Array.isArray(settings)) {
    return { code: 200, msg: "success", data: [] };
  }
  return {
    code: 200,
    msg: "success",
    data: settings.map((item) => {
      const config = parseSettingConfig(item);
      const identifier = item.identifier || item.platformIdentifier || "Local";
      const normalized = normalizeIdentifier(identifier);
      return {
        settingId: item.id || item.settingId || "",
        storageSettingName: item.storageSettingName || item.name || "",
        identifier,
        name: item.storageSettingName || item.name || identifier || "Local",
        active: item.active === true || item.active === 1 || item.enabled === true || item.enabled === 1,
        basePath: config.basePath || config.rootPath || "",
        namespace: config.namespace || "",
        baseUrl: config.baseUrl || config.publicBaseUrl || "",
        endpoint: config.endpoint || "",
        region: config.region || "",
        bucket: config.bucket || "",
        accessKeyId: config.access_key_id || config.accessKeyId || "",
        secretAccessKey: config.secret_access_key || config.secretAccessKey || "",
        prefix: config.prefix || "",
        useSSL: config.use_ssl === true || config.useSSL === true,
        pathStyle: normalized === "s3" ? config.path_style !== false : (config.path_style === true || config.pathStyle === true),
        config,
        raw: item
      };
    })
  };
}

export async function fetchActiveStorage(baseUrl) {
  const listResp = await fetchStorageSettings(baseUrl);
  const settings = unwrapPayload(listResp) || [];
  const active = settings.find((item) => item.active) || settings[0] || null;
  return {
    code: 200,
    msg: "success",
    data: active
  };
}

export async function updateActiveStorage(baseUrl, payload) {
  const identifier = payload.identifier || "Local";
  const normalized = normalizeIdentifier(identifier);
  let config = {};
  if (normalized === "s3") {
    config = {
      endpoint: payload.endpoint || "",
      region: payload.region || "us-east-1",
      bucket: payload.bucket || "",
      access_key_id: payload.accessKeyId || "",
      secret_access_key: payload.secretAccessKey || "",
      prefix: payload.prefix || "",
      use_ssl: Boolean(payload.useSSL),
      path_style: payload.pathStyle !== false
    };
  } else {
    config = {
      namespace: payload.namespace || "",
      baseUrl: payload.baseUrl || ""
    };
  }
  const configJson = JSON.stringify(config);

  let settingId = payload.settingId;
  const storageSettingName = String(payload.storageSettingName || "").trim();
  if (!settingId || settingId === "local-default") {
    const createResp = await requestJson("POST", [
      `${baseUrl}/api/v1/storage/settings`,
      `${baseUrl}/apis/storage/settings`
    ], {
      body: { storageSettingName, identifier, configJson }
    });
    const created = unwrapPayload(createResp) || {};
    settingId = created.id || created.settingId;
  }

  // 可编辑时先更新配置。
  if (settingId && payload.settingId && payload.settingId !== "local-default") {
    try {
      await requestJson("PUT", [
        `${baseUrl}/api/v1/storage/settings/${encodeURIComponent(settingId)}`,
        `${baseUrl}/apis/storage/settings/${encodeURIComponent(settingId)}`
      ], {
        body: { storageSettingName, configJson }
      });
    } catch {
      // 部分后端版本没有更新路由，忽略并继续启用流程。
    }
  }

  try {
    await requestJson("POST", [
      `${baseUrl}/api/v1/storage/settings/${encodeURIComponent(settingId)}/activate`,
      `${baseUrl}/apis/storage/settings/${encodeURIComponent(settingId)}/1`
    ], {});
  } catch {
    const createResp = await requestJson("POST", [
      `${baseUrl}/api/v1/storage/settings`,
      `${baseUrl}/apis/storage/settings`
    ], {
      body: { storageSettingName, identifier, configJson }
    });
    const created = unwrapPayload(createResp) || {};
    settingId = created.id || created.settingId || settingId;
    await requestJson("POST", [
      `${baseUrl}/api/v1/storage/settings/${encodeURIComponent(settingId)}/activate`,
      `${baseUrl}/apis/storage/settings/${encodeURIComponent(settingId)}/1`
    ], {});
  }

  return {
    code: 200,
    msg: "success",
    data: {
      settingId,
      storageSettingName,
      identifier,
      basePath: payload.basePath || "",
      baseUrl: payload.baseUrl || ""
    }
  };
}

export function activateStorageSetting(baseUrl, settingId) {
  return requestJson("POST", [
    `${baseUrl}/api/v1/storage/settings/${encodeURIComponent(settingId)}/activate`,
    `${baseUrl}/apis/storage/settings/${encodeURIComponent(settingId)}/1`
  ], {});
}

export function login(baseUrl, payload) {
  return requestJson("POST", `${baseUrl}/apis/auth/login`, { body: payload, withAuth: false });
}

export function logout(baseUrl) {
  return requestJson("POST", `${baseUrl}/apis/auth/logout`, { body: {} });
}

export function fetchCurrentUser(baseUrl) {
  return requestJson("GET", [
    `${baseUrl}/apis/user/info`,
    `${baseUrl}/apis/auth/me`
  ]);
}

export function fetchWorkspaces(baseUrl) {
  return requestJson("GET", [
    `${baseUrl}/apis/user/workspaces`,
    `${baseUrl}/apis/workspaces`
  ]);
}

export async function fetchActiveWorkspace(baseUrl) {
  const listResp = await fetchWorkspaces(baseUrl);
  const workspaces = unwrapPayload(listResp) || [];
  if (!Array.isArray(workspaces) || workspaces.length === 0) {
    return { code: "OK", message: "success", data: null };
  }
  const active = workspaces.find((item) => item?.isDefault) || workspaces[0];
  return { code: "OK", message: "success", data: active };
}

export function updateActiveWorkspace(baseUrl, payload) {
  const workspaceId = encodeURIComponent(payload?.workspaceId || "");
  return requestJson("PUT", [
    `${baseUrl}/apis/user/default-workspace/${workspaceId}`,
    `${baseUrl}/apis/workspaces/active`
  ], { body: payload });
}

export function fetchFileList(baseUrl) {
  return requestJson("GET", `${baseUrl}/apis/file/list`);
}

export function fetchEntriesByParent(baseUrl, parentId) {
  const encodedParentId = encodeURIComponent(parentId || "");
  return requestJson("GET", [
    `${baseUrl}/apis/files/by-parent?parentId=${encodedParentId}`,
    `${baseUrl}/apis/file/list?parentId=${encodedParentId}`
  ]);
}

export function createDirectory(baseUrl, payload) {
  return requestJson("POST", `${baseUrl}/apis/file/directory`, {
    body: payload,
    withIdempotency: true
  });
}

export function fetchDirectories(baseUrl, parentId) {
  const encodedParentId = encodeURIComponent(parentId || "");
  return requestJson("GET", `${baseUrl}/apis/file/dirs?parentId=${encodedParentId}`);
}

export function renameDirectory(baseUrl, directoryId, name) {
  return requestJson("PUT", `${baseUrl}/apis/file/${encodeURIComponent(directoryId)}/rename`, {
    body: { newName: name },
    withIdempotency: true
  });
}

export function deleteDirectory(baseUrl, directoryId) {
  return requestJson("DELETE", `${baseUrl}/apis/file`, {
    body: [directoryId],
    withIdempotency: true
  });
}

export function renameFile(baseUrl, fileId, name) {
  return requestJson("PUT", `${baseUrl}/apis/file/${encodeURIComponent(fileId)}/rename`, {
    body: { newName: name },
    withIdempotency: true
  });
}

export function moveFile(baseUrl, fileId, targetParentId) {
  return requestJson("PUT", `${baseUrl}/apis/file/moves`, {
    body: { fileIds: [fileId], targetParentId },
    withIdempotency: true
  });
}

export function deleteFile(baseUrl, fileId) {
  return requestJson("DELETE", `${baseUrl}/apis/file`, {
    body: [fileId],
    withIdempotency: true
  });
}

export function fetchRecycleBin(baseUrl) {
  return requestJson("GET", `${baseUrl}/apis/file/recycle/pages?page=1&size=200`);
}

export function restoreRecord(baseUrl, fileId) {
  return requestJson("PUT", `${baseUrl}/apis/file/recycles`, {
    body: [fileId],
    withIdempotency: true
  });
}

export function precheckUpload(baseUrl, payload) {
  return requestJson("POST", [
    `${baseUrl}/apis/transfer/check`,
    `${baseUrl}/apis/upload/precheck`
  ], { body: payload });
}

export async function uploadPart(baseUrl, payload) {
  const taskId = payload.taskId || payload.uploadId;
  const chunkIndex = payload.chunkIndex ?? payload.partNumber;
  const chunkMd5 = payload.chunkMd5 || payload.fileHash || "";

  const formData = new FormData();
  formData.append("taskId", taskId);
  formData.append("uploadId", taskId);
  formData.append("chunkIndex", String(chunkIndex));
  formData.append("partNumber", String(chunkIndex));
  formData.append("totalParts", String(payload.totalParts));
  formData.append("fileHash", payload.fileHash);
  formData.append("file", payload.file);

  const candidates = [
    `${baseUrl}/apis/transfer/chunk?taskId=${encodeURIComponent(taskId)}&chunkIndex=${encodeURIComponent(String(chunkIndex))}&chunkMd5=${encodeURIComponent(chunkMd5)}`,
    `${baseUrl}/apis/upload/part`
  ];

  let lastError = null;
  for (const url of candidates) {
    const response = await fetch(url, {
      method: "POST",
      headers: buildHeaders({}, true, false),
      body: formData
    });

    if (response.status === 404 || response.status === 405) {
      lastError = new Error(`endpoint not found: ${url}`);
      continue;
    }

    const text = await response.text();
    let payloadData = null;
    if (text) {
      try {
        payloadData = JSON.parse(text);
      } catch {
        payloadData = null;
      }
    }

    if (!response.ok) {
      throw new Error(readErrorMessage(payloadData, `分片上传失败(${response.status})`));
    }
    if (payloadData?.success === false) {
      throw new Error(readErrorMessage(payloadData, "分片上传失败"));
    }
    return payloadData;
  }

  throw lastError || new Error("分片上传失败");
}

export function mergeUpload(baseUrl, payload) {
  const taskId = payload.taskId || payload.uploadId;
  return requestJson("POST", [
    `${baseUrl}/apis/transfer/merge/${encodeURIComponent(taskId)}`,
    `${baseUrl}/apis/upload/merge`
  ], { body: payload });
}

export function rebuildFileIndexes(baseUrl) {
  return requestJson("POST", `${baseUrl}/apis/files/maintenance/rebuild-indexes`, { body: {} });
}

export function createShare(baseUrl, payload) {
  return requestJson("POST", [
    `${baseUrl}/apis/share/create`,
    `${baseUrl}/apis/shares`
  ], { body: payload });
}

export function fetchMyShares(baseUrl) {
  return requestJson("GET", [
    `${baseUrl}/apis/share/pages`,
    `${baseUrl}/apis/shares/mine`
  ]);
}

export function accessPublicShare(baseUrl, shareId, shareCode) {
  return requestJson("POST", `${baseUrl}/apis/shares/public/${encodeURIComponent(shareId)}/access`, {
    body: { shareCode },
    withAuth: false
  });
}

export function fetchShareDetail(baseUrl, shareId) {
  return requestJson("GET", [
    `${baseUrl}/apis/share/${encodeURIComponent(shareId)}`,
    `${baseUrl}/apis/shares/${encodeURIComponent(shareId)}`
  ]);
}

export function updateShare(baseUrl, shareId, payload) {
  return requestJson("PUT", [
    `${baseUrl}/apis/share/${encodeURIComponent(shareId)}`,
    `${baseUrl}/apis/shares/${encodeURIComponent(shareId)}`
  ], { body: payload });
}
