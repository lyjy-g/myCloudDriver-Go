import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { getJson, postJson } from "../api/storage.js";

/**
 * 构建模拟响应。
 *
 * @param {boolean} ok 是否成功
 * @param {number} status 状态码
 * @param {any} payload 响应数据
 * @returns {Response} 模拟响应
 */
function buildResponse(ok, status, payload) {
  return {
    ok,
    status,
    /**
     * 返回 JSON 数据。
     *
     * @returns {Promise<any>} JSON 数据
     */
    json: async () => payload,
    /**
     * 返回文本。
     *
     * @returns {Promise<string>} 文本
     */
    text: async () => JSON.stringify(payload)
  };
}

/**
 * 存储 API 测试。
 */
describe("storage api", () => {
  /**
   * 每个用例前清理 mock。
   */
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  /**
   * 每个用例后清理 mock。
   */
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });
  /**
   * 成功返回 JSON。
   */
  it("getJson returns payload when ok", async () => {
    const payload = { hello: "world" };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(buildResponse(true, 200, payload)));

    const result = await getJson("http://localhost:8080/apis/test");

    expect(result).toEqual(payload);
  });

  /**
   * 失败时抛出错误。
   */
  it("getJson throws when not ok", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(buildResponse(false, 500, { error: "boom" })));

    await expect(getJson("http://localhost:8080/apis/test")).rejects.toThrow("boom");
  });

  /**
   * postJson 成功返回数据。
   */
  it("postJson returns payload when ok", async () => {
    const payload = { ok: true };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(buildResponse(true, 200, payload)));

    const result = await postJson("http://localhost:8080/apis/post", { name: "demo" });

    expect(result).toEqual(payload);
  });
});
