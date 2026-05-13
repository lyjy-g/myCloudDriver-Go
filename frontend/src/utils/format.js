/**
 * 格式化文件大小。
 *
 * @param {number} bytes 字节数
 * @returns {string} 可读格式
 */
export function formatBytes(bytes) {
  if (!Number.isFinite(bytes)) {
    return "-";
  }
  const units = ["B", "KB", "MB", "GB"];
  let size = bytes;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  return `${size.toFixed(1)} ${units[unitIndex]}`;
}
