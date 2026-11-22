package nodesync

import "github.com/altercpre/aparok-sync-node/nodesync/hotsync"

type configGetter interface {
	GetNodeSync() Config
}

type Config struct {
	SyncOnStart       bool           `yaml:"syncOnStart"`
	PeriodicSyncHours int            `yaml:"periodicSyncHours"`
	HotSync           hotsync.Config `yaml:"hotSync"`
}
