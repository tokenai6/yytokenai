package utils

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Snowflake 雪花算法生成器
// 标准雪花算法结构（二进制位）：
// - 41位：毫秒级时间戳（从起始时间开始）
// - 10位：机器ID（数据中心ID 5位 + 机器ID 5位）
// - 12位：序列号（同一毫秒内的序号，0-4095）
// 总共64位二进制（bit），转换为十进制约为18-20位数字（digit）
// 使用big.Int确保在uint256范围内且为纯数字
type Snowflake struct {
	mu            sync.Mutex // 互斥锁，保证线程安全
	startTime     int64      // 起始时间（毫秒），用于计算相对时间戳
	datacenterID  int64      // 数据中心ID（0-31，5位）
	workerID      int64      // 机器ID（0-31，5位）
	sequence      int64      // 序列号（0-4095，12位）
	lastTimestamp int64      // 上次生成ID的时间戳（毫秒）
}

var (
	snowflakeInstance *Snowflake
	snowflakeOnce     sync.Once
)

// getSnowflakeInstance 获取雪花算法单例实例
func getSnowflakeInstance() *Snowflake {
	snowflakeOnce.Do(func() {
		ctx := context.Background()

		// 从配置读取数据中心ID和机器ID
		datacenterID := g.Cfg().MustGet(ctx, "snowflake.datacenter_id", 0).Int64()
		workerID := g.Cfg().MustGet(ctx, "snowflake.worker_id", 0).Int64()

		// 验证ID范围
		if datacenterID < 0 || datacenterID > 31 {
			g.Log().Warningf(ctx, "snowflake.datacenter_id 超出范围(0-31)，使用默认值0")
			datacenterID = 0
		}
		if workerID < 0 || workerID > 31 {
			g.Log().Warningf(ctx, "snowflake.worker_id 超出范围(0-31)，使用默认值0")
			workerID = 0
		}

		// 设置起始时间（2024-01-01 00:00:00 UTC）
		startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

		snowflakeInstance = &Snowflake{
			startTime:     startTime,
			datacenterID:  datacenterID,
			workerID:      workerID,
			sequence:      0,
			lastTimestamp: -1,
		}

		g.Log().Infof(ctx, "雪花算法初始化完成: datacenter_id=%d, worker_id=%d, start_time=%d",
			datacenterID, workerID, startTime)
	})

	return snowflakeInstance
}

// GenerateSnowflakeId 生成雪花ID（完整算法，支持多机器部署）
// 返回纯数字字符串，满足智能合约 uint256 类型要求
// 算法结构：41位时间戳 + 10位机器ID + 12位序列号 = 64位
// 使用big.Int确保在uint256范围内（78位十进制数）
func GenerateSnowflakeId() string {
	sf := getSnowflakeInstance()

	sf.mu.Lock()
	defer sf.mu.Unlock()

	// 获取当前时间戳（毫秒）
	timestamp := time.Now().UnixMilli()

	// 如果当前时间小于上次时间戳，说明时钟回拨
	if timestamp < sf.lastTimestamp {
		// 时钟回拨，等待直到时间追上
		offset := sf.lastTimestamp - timestamp
		time.Sleep(time.Duration(offset) * time.Millisecond)
		timestamp = time.Now().UnixMilli()
	}

	// 如果是同一毫秒内
	if timestamp == sf.lastTimestamp {
		// 序列号自增
		sf.sequence = (sf.sequence + 1) & 0xFFF // 12位序列号，最大4095

		// 如果序列号溢出（同一毫秒内超过4096个ID）
		if sf.sequence == 0 {
			// 等待下一毫秒
			timestamp = sf.waitNextMillis(sf.lastTimestamp)
		}
	} else {
		// 新的毫秒，序列号重置为0
		sf.sequence = 0
	}

	// 更新上次时间戳
	sf.lastTimestamp = timestamp

	// 计算相对时间戳（从起始时间开始的毫秒数）
	relativeTimestamp := timestamp - sf.startTime

	// 使用big.Int构建64位ID
	// 结构：41位时间戳 + 10位机器ID + 12位序列号
	id := big.NewInt(relativeTimestamp)

	// 左移10位，为机器ID腾出空间
	id.Lsh(id, 10)

	// 添加机器ID（5位数据中心ID + 5位机器ID）
	machineID := (sf.datacenterID << 5) | sf.workerID
	id.Add(id, big.NewInt(machineID))

	// 左移12位，为序列号腾出空间
	id.Lsh(id, 12)

	// 添加序列号
	id.Add(id, big.NewInt(sf.sequence))

	// 验证：确保结果在 uint256 范围内（小于 2^256）
	maxUint256 := new(big.Int)
	maxUint256.Exp(big.NewInt(2), big.NewInt(256), nil)
	maxUint256.Sub(maxUint256, big.NewInt(1))

	// 理论上不会超出范围（64位ID远小于uint256），但为了安全起见
	if id.Cmp(maxUint256) > 0 {
		// 如果超出，使用取模运算（理论上不会发生）
		id.Mod(id, maxUint256)
	}

	return id.String()
}

// waitNextMillis 等待下一毫秒
func (sf *Snowflake) waitNextMillis(lastTimestamp int64) int64 {
	timestamp := time.Now().UnixMilli()
	for timestamp <= lastTimestamp {
		timestamp = time.Now().UnixMilli()
	}
	return timestamp
}
