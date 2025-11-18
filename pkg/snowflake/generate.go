package snowflake

import (
	"errors"
	"sync"

	"github.com/bwmarrin/snowflake"
)

var (
	node *snowflake.Node
	once sync.Once

	errInvalidMachineID   = errors.New("invalid snowflake machine id")
	errGeneratorUninitial = errors.New("snowflake generator is not initialized")
)

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
