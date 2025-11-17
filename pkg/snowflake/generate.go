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

// getMachineID 根据服务名称自动分配 machine ID
func getMachineID() int64 {
	cfg := config.Cfg


	if cfg.SnowflakeMachineID > 0 {
		return cfg.SnowflakeMachineID
	}


	serviceName := cfg.ServiceName
	switch serviceName {
	case "areyouok-api", "areyouok":
		return 1 
	case "areyouok-worker": //还需考虑多个 worker 实例的情况，到时候具体配置
		return 2 
	default:
		
		return 1
	}
}

func Init(machineID, dataCenterID int64) error {
	var initErr error

	once.Do(func() {

	

		if machineID < 0 || machineID > 31 {
			initErr = errInvalidMachineID
			return
		}
		nodeID := (dataCenterID << 5) | machineID // datacenterID 和 machineID 都是 0~31

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
