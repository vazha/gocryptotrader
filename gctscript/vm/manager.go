package vm

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/vazha/gocryptotrader/engine/subsystem"
	"github.com/vazha/gocryptotrader/log"
)

const gctscriptManagerName = "GCTScript"

// GctScriptManager loads and runs GCT Tengo scripts
type GctScriptManager struct {
	config   *Config
	started  int32
	stopped  int32
	shutdown chan struct{}
	// Optional values to override stored config ('nil' if not overridden)
	MaxVirtualMachines *uint8
}

// NewManager creates a new instance of script manager
func NewManager(config *Config) (*GctScriptManager, error) {
	if config == nil {
		return nil, errors.New("config must be provided for script manager")
	}
	return &GctScriptManager{
		config: config,
	}, nil
}

// Started returns if gctscript manager subsystem is started
func (g *GctScriptManager) Started() bool {
	return atomic.LoadInt32(&g.started) == 1
}

// Start starts gctscript subsystem and creates shutdown channel
func (g *GctScriptManager) Start(wg *sync.WaitGroup) (err error) {
	if atomic.AddInt32(&g.started, 1) != 1 {
		return fmt.Errorf("%s %s", gctscriptManagerName, subsystem.ErrSubSystemAlreadyStarted)
	}

	defer func() {
		if err != nil {
			atomic.CompareAndSwapInt32(&g.started, 1, 0)
		}
	}()
	log.Debugln(log.Global, gctscriptManagerName, subsystem.MsgSubSystemStarting)

	g.shutdown = make(chan struct{})
	wg.Add(1)
	go g.run(wg)
	return nil
}

// Stop stops gctscript subsystem along with all running Virtual Machines
func (g *GctScriptManager) Stop() error {
	if atomic.LoadInt32(&g.started) == 0 {
		return fmt.Errorf("%s %s", gctscriptManagerName, subsystem.ErrSubSystemNotStarted)
	}

	if atomic.AddInt32(&g.stopped, 1) != 1 {
		return fmt.Errorf("%s %s", gctscriptManagerName, subsystem.ErrSubSystemAlreadyStopped)
	}

	log.Debugln(log.GCTScriptMgr, gctscriptManagerName, subsystem.MsgSubSystemShuttingDown)
	close(g.shutdown)
	err := g.ShutdownAll()
	if err != nil {
		return err
	}
	return nil
}

func (g *GctScriptManager) run(wg *sync.WaitGroup) {
	log.Debugln(log.Global, gctscriptManagerName, subsystem.MsgSubSystemStarted)

	SetDefaultScriptOutput()
	g.autoLoad()
	defer func() {
		atomic.CompareAndSwapInt32(&g.stopped, 1, 0)
		atomic.CompareAndSwapInt32(&g.started, 1, 0)
		wg.Done()
		log.Debugln(log.GCTScriptMgr, gctscriptManagerName, subsystem.MsgSubSystemShutdown)
	}()

	<-g.shutdown
}

// GetMaxVirtualMachines returns the max number of VMs to create
func (g *GctScriptManager) GetMaxVirtualMachines() uint8 {
	if g.MaxVirtualMachines != nil {
		return *g.MaxVirtualMachines
	}
	return g.config.MaxVirtualMachines
}
