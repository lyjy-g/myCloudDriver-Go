import {
  CloudUploadOutlined,
  CustomerServiceOutlined,
  DeleteOutlined,
  FileTextOutlined,
  FolderOpenOutlined,
  HomeOutlined,
  InboxOutlined,
  PictureOutlined,
  ShareAltOutlined,
  SettingOutlined,
  StarOutlined,
  ToolOutlined,
  VideoCameraOutlined
} from "@ant-design/icons";

/**
 * 本地存储键名。
 */
export const STORAGE_KEY = "mcd-console-base-url";

/**
 * 默认分片大小（5MB）。
 */
export const DEFAULT_CHUNK_SIZE = 5 * 1024 * 1024;

/**
 * 根目录 ID。
 */
export const ROOT_PARENT_ID = "ROOT";

/**
 * 侧栏菜单。
 */
export const SIDEBAR_MENU_ITEMS = [
  { key: "files", icon: FolderOpenOutlined, label: "全部文件" },
  { key: "shares", icon: ShareAltOutlined, label: "我的分享" },
  { key: "trash", icon: DeleteOutlined, label: "回收站" },
  { key: "settings", icon: SettingOutlined, label: "存储配置" }
];

/**
 * 快捷操作配置。
 */
export const QUICK_ACTIONS = [
  { key: "upload", label: "上传", icon: CloudUploadOutlined },
  { key: "new-folder", label: "新建文件夹", icon: FolderOpenOutlined },
  { key: "new-note", label: "新建笔记", icon: FileTextOutlined }
];

/**
 * 顶部标签配置。
 */
export const TOP_PROMOS = [
  { key: "team", label: "MyCloudDrive", tone: "pill" },
  { key: "vip", label: "现代化网盘", tone: "warm" },
  { key: "sale", label: "个人/组织", tone: "outline" }
];

/**
 * 文件分类配置。
 */
export const TYPE_FILTERS = [
  { key: "images", label: "图片", icon: PictureOutlined },
  { key: "docs", label: "文档", icon: FileTextOutlined },
  { key: "videos", label: "视频", icon: VideoCameraOutlined },
  { key: "torrents", label: "种子", icon: InboxOutlined },
  { key: "audio", label: "音频", icon: CustomerServiceOutlined },
  { key: "others", label: "其它", icon: ToolOutlined }
];

/**
 * 图标栏按钮。
 */
export const ICON_RAIL_ITEMS = [
  { key: "home", icon: HomeOutlined, label: "首页" },
  { key: "files", icon: FolderOpenOutlined, label: "文件", active: true },
  { key: "inbox", icon: InboxOutlined, label: "收发" },
  { key: "message", icon: CustomerServiceOutlined, label: "消息" },
  { key: "tools", icon: ToolOutlined, label: "工具" },
  { key: "favorite", icon: StarOutlined, label: "收藏" }
];

/**
 * 本地路径快捷预设。
 */
export const LOCAL_PATH_PRESETS = [
  { label: "使用 .data", value: "/home/lyjy/code/MyCloudDrive/.data/files" },
  { label: "使用 runtime-data", value: "/home/lyjy/code/MyCloudDrive/runtime-data/files" }
];
