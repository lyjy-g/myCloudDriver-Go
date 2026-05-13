import { useCallback, useMemo, useState } from "react";
import { STORAGE_KEY } from "../constants/appConfig.js";

/**
 * 获取默认服务端地址。
 *
 * @returns {string} 默认地址
 */
function getDefaultBaseUrl() {
  return localStorage.getItem(STORAGE_KEY) || "http://localhost:8080";
}

/**
 * 服务端地址状态。
 *
 * @returns {{baseUrl: string, normalizedBaseUrl: string, handleBaseUrlChange: Function, saveBaseUrl: Function}}
 */
export function useBaseUrl() {
  const [baseUrl, setBaseUrl] = useState(getDefaultBaseUrl);

  const normalizedBaseUrl = useMemo(() => baseUrl.trim().replace(/\/$/, ""), [baseUrl]);

  const handleBaseUrlChange = useCallback((event) => {
    setBaseUrl(event.target.value);
  }, []);

  const saveBaseUrl = useCallback(() => {
    const trimmed = baseUrl.trim();
    if (!trimmed) {
      return false;
    }
    const nextValue = trimmed.replace(/\/$/, "");
    setBaseUrl(nextValue);
    localStorage.setItem(STORAGE_KEY, nextValue);
    return true;
  }, [baseUrl]);

  return {
    baseUrl,
    normalizedBaseUrl,
    handleBaseUrlChange,
    saveBaseUrl
  };
}
