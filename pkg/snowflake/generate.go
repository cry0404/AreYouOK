package snowflake

import (
	"errors"
	"sync"

	"github.com/bwmarrin/snowflake"

	"AreYouOK/config"
)

var (
	node *snowflake.Node
	once sync.Once

	errInvalidMachineID   = errors.New("invalid snowflake machine id")
	errInvalidDatacenter  = errors.New("invalid snowflake datacenter id")
	errGeneratorUninitial = errors.New("snowflake generator is not initialized")
)

func Init() error {
	var initErr error

	once.Do(func() {
		cfg := config.Cfg

		if cfg.SnowflakeMachineID < 0 || cfg.SnowflakeMachineID > 31 {
			initErr = errInvalidMachineID
		}
		if cfg.SnowflakeDataCenter < 0 || cfg.SnowflakeDataCenter > 31 {
			initErr = errInvalidDatacenter
			return
		}
		datacenterID := cfg.SnowflakeDataCenter
		machineID := cfg.SnowflakeMachineID
		nodeID := (datacenterID << 5) | machineID // datacenterID 和 machineID 都是 0~31
		var err error
		node, err = snowflake.NewNode(nodeID)

		if err != nil {
			initErr = err
			return
		}
	})

	return initErr
}

func NextID() (int64, error) {
	if node == nil {
		return 0, errGeneratorUninitial
	}

	return node.Generate().Int64(), nil
}
