package entity

import (
	"XWFrame/internal/frame/model"

	"github.com/gogf/gf/v2/os/gtime"
)

// AdminOperationLogEntity 后台管理操作日志实体
type AdminOperationLogEntity struct {
	model.BaseEntity
	OperatorId      int64       `json:"operator_id" g:"column:operator_id"`
	OperatorName    string      `json:"operator_name" g:"column:operator_name"`
	OperationTime   *gtime.Time `json:"operation_time" g:"column:operation_time"`
	OperationType   string      `json:"operation_type" g:"column:operation_type"`
	RequestData     string      `json:"request_data" g:"column:request_data"` // JSON格式字符串
	OperationResult string      `json:"operation_result" g:"column:operation_result"`
	ErrorMessage    string      `json:"error_message" g:"column:error_message"`
}
