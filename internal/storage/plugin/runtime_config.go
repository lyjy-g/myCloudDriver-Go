package plugin

type ResolvedStorageConfig struct {
	SettingID          string
	UserID             string
	PlatformIdentifier PlatformIdentifier
	ConfigData         []byte
	Version            string
}
