package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/altercpre/aparok-sync/app/debugstat"
	"github.com/altercpre/aparok-sync/commonspace/credentialprovider"
	"github.com/altercpre/aparok-sync/consensus/consensusclient"
	"github.com/altercpre/aparok-sync/coordinator/coordinatorclient"
	"github.com/altercpre/aparok-sync/coordinator/nodeconfsource"
	"github.com/altercpre/aparok-sync/metric"
	"github.com/altercpre/aparok-sync/net/peerservice"
	"github.com/altercpre/aparok-sync/net/pool"
	"github.com/altercpre/aparok-sync/net/rpc/debugserver"
	"github.com/altercpre/aparok-sync/net/rpc/server"
	"github.com/altercpre/aparok-sync/net/streampool"
	"github.com/altercpre/aparok-sync/net/transport/quic"
	"github.com/altercpre/aparok-sync/net/transport/yamux"
	"github.com/altercpre/aparok-sync/node/nodeclient"
	"github.com/altercpre/aparok-sync/nodeconf"
	"github.com/altercpre/aparok-sync/nodeconf/nodeconfstore"
	"github.com/altercpre/aparok-sync/util/syncqueues"

	"github.com/altercpre/aparok-sync-node/archive"
	"github.com/altercpre/aparok-sync-node/archive/adopter"
	"github.com/altercpre/aparok-sync-node/archive/archivestore"
	"github.com/altercpre/aparok-sync-node/nodehead"
	"github.com/altercpre/aparok-sync-node/nodespace/migrator"
	"github.com/altercpre/aparok-sync-node/nodespace/peermanager"
	"github.com/altercpre/aparok-sync-node/nodespace/spacedeleter"
	"github.com/altercpre/aparok-sync-node/nodesync"
	"github.com/altercpre/aparok-sync-node/nodesync/coldsync"
	"github.com/altercpre/aparok-sync-node/nodesync/hotsync"
	"github.com/altercpre/aparok-sync-node/oldstorage"
	"github.com/altercpre/aparok-sync-node/repairer"
	"github.com/altercpre/aparok-sync-node/resharder"

	// import this to keep govvv in go.mod on mod tidy
	_ "github.com/ahmetb/govvv/integration-test/app-different-package/mypkg"
	"github.com/altercpre/aparok-sync/app"
	"github.com/altercpre/aparok-sync/app/logger"
	"github.com/altercpre/aparok-sync/commonspace"
	"github.com/altercpre/aparok-sync/net/secureservice"
	"go.uber.org/zap"

	"github.com/altercpre/aparok-sync-node/account"
	"github.com/altercpre/aparok-sync-node/config"
	"github.com/altercpre/aparok-sync-node/debug/nodedebugrpc"
	"github.com/altercpre/aparok-sync-node/debug/spacechecker"
	"github.com/altercpre/aparok-sync-node/nodespace"
	"github.com/altercpre/aparok-sync-node/nodespace/nodecache"
	"github.com/altercpre/aparok-sync-node/nodespace/pubsubrelay"
	"github.com/altercpre/aparok-sync-node/nodestorage"
)

var log = logger.NewNamed("main")

var (
	flagConfigFile = flag.String("c", "etc/aparok-sync-node.yml", "path to config file")
	flagVersion    = flag.Bool("v", false, "show version and exit")
	flagHelp       = flag.Bool("h", false, "show help and exit")
)

func main() {
	flag.Parse()

	if *flagVersion {
		fmt.Println(app.AppName)
		fmt.Println(app.Version())
		fmt.Println(app.VersionDescription())
		return
	}
	if *flagHelp {
		flag.PrintDefaults()
		return
	}

	if debug, ok := os.LookupEnv("ANYPROF"); ok && debug != "" {
		go func() {
			http.ListenAndServe(debug, nil)
		}()
	}

	// create app
	ctx := context.Background()
	a := new(app.App)

	// open config file
	conf, err := config.NewFromFile(*flagConfigFile)
	if err != nil {
		log.Fatal("can't open config file", zap.Error(err))
	}

	// bootstrap components
	a.Register(conf)
	Bootstrap(a)

	// start app
	if err := a.Start(ctx); err != nil {
		log.Fatal("can't start app", zap.Error(err))
	}
	log.Info("app started", zap.String("version", a.Version()))

	// wait exit signal
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-exit
	log.Info("received exit signal, stop app...", zap.String("signal", fmt.Sprint(sig)))

	// close app
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := a.Close(ctx); err != nil {
		log.Fatal("close error", zap.Error(err))
	} else {
		log.Info("goodbye!")
	}
	time.Sleep(time.Second / 3)
}

func Bootstrap(a *app.App) {
	a.Register(account.New()).
		Register(metric.New()).
		Register(debugstat.New()).
		Register(credentialprovider.NewNoOp()).
		Register(coordinatorclient.New()).
		Register(nodeconfstore.New()).
		Register(nodeconfsource.New()).
		Register(nodeconf.New()).
		Register(oldstorage.New()).
		Register(nodestorage.New()).
		Register(migrator.New()).
		Register(syncqueues.New()).
		Register(server.New()).
		Register(peerservice.New()).
		Register(pool.New()).
		Register(nodeclient.New()).
		Register(consensusclient.New()).
		Register(nodespace.NewStreamOpener()).
		Register(streampool.New()).
		Register(nodehead.New()).
		Register(nodecache.New(200)).
		Register(hotsync.New()).
		Register(coldsync.New()).
		Register(nodesync.New()).
		Register(secureservice.New()).
		Register(commonspace.New()).
		Register(nodespace.New()).
		Register(pubsubrelay.New()).
		Register(spacedeleter.New()).
		Register(peermanager.New()).
		Register(debugserver.New()).
		Register(spacechecker.New()).
		Register(nodedebugrpc.New()).
		Register(archivestore.New()).
		Register(archive.New()).
		Register(adopter.New()).
		Register(resharder.New()).
		Register(repairer.New()).
		Register(quic.New()).
		Register(yamux.New())
}
