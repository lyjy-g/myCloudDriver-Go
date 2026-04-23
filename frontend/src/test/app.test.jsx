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
    if (url.includes("/apis/storage/active")) {
      return buildResponse({
        success: true,
        data: {
          settingId: "local-default",
          identifier: "Local",
          name: "本地存储",
          basePath: "/tmp/files",
          baseUrl: "http://localhost:8080/files"
        }
      });
    }
    if (url.includes("/apis/workspaces/active")) {
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
    if (url.includes("/apis/workspaces")) {
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
    if (url.includes("/apis/files/by-parent?parentId=ROOT")) {
      return buildResponse({ success: true, data: [] });
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

  it("shows toolbar and can open settings", async () => {
    render(<App />);

    await screen.findByText("刷新");
    const switchButton = screen.getByText("切换");
    fireEvent.click(switchButton);

    await screen.findByText("当前配置");
  });

  it("fetches file list when clicking refresh", async () => {
    render(<App />);
    const refreshButtons = await screen.findAllByText("刷新");
    fireEvent.click(refreshButtons[0]);

    await waitFor(() => {
      const calledByParent = global.fetch.mock.calls.some(
        ([url]) => typeof url === "string" && url.includes("/apis/files/by-parent?parentId=ROOT")
      );
      expect(calledByParent).toBe(true);
    });
  });
});
