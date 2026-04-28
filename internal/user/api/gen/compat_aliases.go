package userapi

// 兼容别名：将历史业务层使用的类型名映射到当前 OpenAPI 生成类型。
// 目的：在统一 OpenAPI Go 风格命名后，避免大面积改动 service/handler 代码。
type SysUserVO = SysUserResponse
type UserRegisterCmd = UserRegisterRequest
type UserEditInfoCmd = UserEditInfoRequest
type PasswordEditCmd = PasswordEditRequest
type PasswordForgetEditCmd = PasswordForgetEditRequest
type UserTransferSettingEditCmd = UserTransferSettingEditRequest
