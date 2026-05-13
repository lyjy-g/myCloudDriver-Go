import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import React from "react";
import App from "../App.jsx";

/**
 * 构建模拟响应。
 */
function buildResponse(payload) {
  return {
    ok: true,
    status: 200,
    json: async () => payload,
    text: async () => JSON.stringify(payload)
  };
}

/**
 * 统一 mock 已登录态接口。
 */
function buildAuthedFetchMock() {
  return vi.fn(async (url) => {
    if (typeof url !== "string") {
      return buildResponse({ success: true, data: {} });
    }
    if (url.includes("/apis/auth/me")) {
      return buildResponse({
        success: true,
        data: { username: "myCloudDrive", displayName: "MyCloudDrive 管理员" }
      });
    }
    if (url.includes("/apis/storage/platforms")) {
      return buildResponse({ success: true, data: [{ identifier: "Local", name: "本地存储", isDefault: true }] });
    }
    if (url.includes("/apis/user/workspace/active")) {
      return buildResponse({
        success: true,
        data: {
          workspaceId: "ws-personal-default",
          workspaceName: "我的空间",
          storageIdentifier: "Local",
          storageSettingId: "local-default"
        }
      });
    }
    if (url.includes("/apis/user/workspaces")) {
      return buildResponse({
        success: true,
        data: [
          {
            workspaceId: "ws-personal-default",
            workspaceName: "我的空间",
            storageIdentifier: "Local",
            storageSettingId: "local-default"
          }
        ]
      });
    }
    if (url.includes("/apis/storage/platform/settings")) {
      return buildResponse({
        success: true,
        data: [{ id: "local-default", storageSettingName: "本地存储", identifier: "Local", active: true, configJson: "{}" }]
      });
    }
    if (url.includes("/apis/file/list?parentId=root")) {
      return buildResponse({ success: true, data: { items: [], total: 0 } });
    }
    return buildResponse({ success: true, data: {} });
  });
}

describe("App", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
    vi.stubGlobal("fetch", buildAuthedFetchMock());
  });

  afterEach(() => {
    localStorage.clear();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("shows toolbar and workspace section", async () => {
    render(<App />);

    await screen.findByText("刷新");
    await screen.findByText("空间列表");
  });

  it("fetches file list when clicking refresh", async () => {
    render(<App />);
    const refreshButtons = await screen.findAllByText("刷新");
    fireEvent.click(refreshButtons[0]);

    await waitFor(() => {
      const calledByParent = global.fetch.mock.calls.some(
        ([url]) => typeof url === "string" && url.includes("/apis/file/list?parentId=root")
      );
      expect(calledByParent).toBe(true);
    });
  });
});
