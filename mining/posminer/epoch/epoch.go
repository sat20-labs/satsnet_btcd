package epoch

import (
	"time"

	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validator"
)

type Epoch struct {
	CreateHeight  uint64                 // 当前Epoch的创建高度
	CreateTime    time.Time              // 当前Epoch的创建时间
	ValidatorList []*validator.Validator // 所有的验证者列表
	Generator     *validator.Validator   // 当前Slot的生成者
	CurrentHeight uint64                 // 当前Block chain的高度
}
