package oldstorage

import "github.com/altercpre/aparok-sync-node/nodestorage"

type configGetter interface {
	GetStorage() nodestorage.Config
}
