/**
 * 计算文件哈希（SHA-256）。
 *
 * @param {File} file 文件对象
 * @returns {Promise<string>} 哈希值
 */
export async function calculateHash(file) {
  const buffer = await file.arrayBuffer();
  const digest = await crypto.subtle.digest("SHA-256", buffer);
  const view = new DataView(digest);
  let hex = "";
  for (let i = 0; i < view.byteLength; i += 1) {
    const value = view.getUint8(i).toString(16).padStart(2, "0");
    hex += value;
  }
  return hex;
}
