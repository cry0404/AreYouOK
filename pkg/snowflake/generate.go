package snowflake

import (
	"errors"
	"fmt"
	"sync"

	"github.com/bwmarrin/snowflake"
)

// GeneratorType ID 生成器类型
type GeneratorType string

const (
	GeneratorTypeUser    GeneratorType = "user"    // 用户 ID
	GeneratorTypeJourney GeneratorType = "journey" // 行程 ID
	GeneratorTypeTask    GeneratorType = "task"    // 通知任务 ID
	GeneratorTypeMessage GeneratorType = "message" // 消息队列 ID
	GeneratorTypeCheckIn GeneratorType = "checkin" // 打卡记录 ID
	GeneratorTypeContact GeneratorType = "contact" // 联系人 ID（如果需要）
)

var (
	generators map[GeneratorType]*snowflake.Node
	genMutex   sync.RWMutex
	baseNodeID int64

	errInvalidMachineID   = errors.New("invalid snowflake machine id")
	errGeneratorUninitial = errors.New("snowflake generator is not initialized")
	errInvalidType        = errors.New("invalid generator type")
)

// Init 初始化 Snowflake 生成器
// machineID: 0-31, dataCenterID: 0-31
// 为每种类型分配不同的 node ID（在 baseNodeID 基础上偏移）
func Init(machineID, dataCenterID int64) error {
	genMutex.Lock()
	defer genMutex.Unlock()

	if generators != nil {
		// 已经初始化，直接返回
		return nil
	}

	if machineID < 0 || machineID > 31 {
		return errInvalidMachineID
	}

	if dataCenterID < 0 || dataCenterID > 31 {
		return fmt.Errorf("invalid data center ID: must be 0-31, got %d", dataCenterID)
	}

	baseNodeID = (dataCenterID << 5) | machineID

	// 为每种类型分配不同的 node ID（在 baseNodeID 基础上偏移）
	// Snowflake 支持最多 1024 个节点（32 datacenter * 32 machine）
	// 我们为每种类型分配一个 node ID
	generators = make(map[GeneratorType]*snowflake.Node)

	nodeIDs := map[GeneratorType]int64{
		GeneratorTypeUser:    baseNodeID,     // 偏移 0
		GeneratorTypeJourney: baseNodeID + 1, // 偏移 1
		GeneratorTypeTask:    baseNodeID + 2, // 偏移 2
		GeneratorTypeMessage: baseNodeID + 3, // 偏移 3
		GeneratorTypeCheckIn: baseNodeID + 4, // 偏移 4
		GeneratorTypeContact: baseNodeID + 5, // 偏移 5
	}

	for genType, nodeID := range nodeIDs {
		// 检查 nodeID 是否超出范围（0-1023）
		if nodeID < 0 || nodeID > 1023 {
			return fmt.Errorf("node ID %d for type %s is out of range (0-1023)", nodeID, genType)
		}

		node, err := snowflake.NewNode(nodeID)
		if err != nil {
			return fmt.Errorf("failed to create node for %s (nodeID=%d): %w", genType, nodeID, err)
		}
		generators[genType] = node
	}

	return nil
}

// NextID 生成指定类型的 ID
func NextID(genType GeneratorType) (int64, error) {
	genMutex.RLock()
	defer genMutex.RUnlock()

	if generators == nil {
		return 0, errGeneratorUninitial
	}

	node, exists := generators[genType]
	if !exists {
		return 0, fmt.Errorf("%w: %s", errInvalidType, genType)
	}

	return node.Generate().Int64(), nil
}

// NextIDString 生成指定类型的 ID（字符串格式）
func NextIDString(genType GeneratorType) (string, error) {
	id, err := NextID(genType)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", id), nil
}

// GetTypeFromID 从 ID 推断类型（通过 node ID 范围）
// 注意：这个方法只能大致推断，因为 Snowflake ID 的结构是：
// 41 bits timestamp + 10 bits node ID + 12 bits sequence
// 我们可以通过 node ID 部分来推断类型
func GetTypeFromID(id int64) GeneratorType {
	genMutex.RLock()
	defer genMutex.RUnlock()

	if generators == nil {
		return ""
	}

	// 提取 node ID（Snowflake ID 的第 42-51 位）
	// ID 结构：timestamp(41) + node(10) + sequence(12)
	nodeID := (id >> 12) & 0x3FF // 提取 10 bits node ID

	// 计算相对于 baseNodeID 的偏移
	offset := nodeID - baseNodeID

	switch offset {
	case 0:
		return GeneratorTypeUser
	case 1:
		return GeneratorTypeJourney
	case 2:
		return GeneratorTypeTask
	case 3:
		return GeneratorTypeMessage
	case 4:
		return GeneratorTypeCheckIn
	case 5:
		return GeneratorTypeContact
	default:
		return "" // 未知类型
	}
}

// // NextIDDefault 保持向后兼容：默认使用 User 类型
//
// func NextIDDefault() (int64, error) {
// 	return NextID(GeneratorTypeUser)
// }
