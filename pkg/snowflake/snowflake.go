package snowflake

import (
	"fmt"
	"sync"

	sf "github.com/bwmarrin/snowflake"
)

var (
	node *sf.Node
	mu   sync.Mutex
)

// Init initializes the snowflake node with a given nodeID
func Init(nodeID int64) error {
	mu.Lock()
	defer mu.Unlock()

	if node != nil {
		return nil
	}

	n, err := sf.NewNode(nodeID)
	if err != nil {
		return err
	}
	node = n
	return nil
}

// GenID generates a unique ID
func GenID() int64 {
	if node == nil {
		// 尝试懒加载初始化（使用默认 NodeID 1）
		if err := Init(1); err != nil {
			panic(fmt.Sprintf("snowflake: lazy init failed: %v", err))
		}
	}
	
	if node == nil {
		panic("snowflake: node is nil (initialization failed)")
	}
	
	return node.Generate().Int64()
}

// GenStringID generates a unique ID as string
func GenStringID() string {
	if node == nil {
		if err := Init(1); err != nil {
			panic(fmt.Sprintf("snowflake: lazy init failed: %v", err))
		}
	}
	
	if node == nil {
		panic("snowflake: node is nil (initialization failed)")
	}
	
	return node.Generate().String()
}
